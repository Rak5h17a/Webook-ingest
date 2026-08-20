# Solution

## What was broken

**1. Duplicate call records / inflated stats**

The original `Ingest` function checked if an event existed, and *then* inserted it which were two separate steps. If the same `event_id` arrived twice close together (which can happen), both requests could check "does this exist?" before either one had actually written its row. Both would see "no" and both would proceed, so the same call got counted twice.

Looking at the migration file confirmed this `events.event_id` only had a regular index but not a `UNIQUE` constraint, so the database wasn't stopping this either. Nothing was actually enforcing uniqueness anywhere.

**2. Recordings never marked processed, so no errors in the logs**

Recording processing runs in a goroutine so the webhook response isn't held up. The problem is that goroutine was using the *request's* context, and Go cancels that context as soon as the HTTP handler returns, which happens almost immediately, well before the goroutine's simulated work finishes. So `MarkRecordingProcessed` was always failing with a context canceled error.

Except you'd never know that, because the error was just thrown away (`// TODO: handle`). That explains why ops saw nothing in the logs as there was nothing logging it.

**3. Stats disappearing after every deploy**

`GET /accounts/{id}/stats` only ever read from the in memory cache. That cache is a plain Go map that gets recreated empty every time the process starts. So the numbers looked fine right up until a deploy, and then reset to zero even though Postgres (`account_stats`) had the correct totals the whole time. The endpoint just never looked there.

**Bonus find:** while I was in `cache.go` fixing #3, I noticed `Cache.Record` wasn't holding the mutex before touching the map, even though `Cache.Get` was. Since multiple webhook requests hit this concurrently, that's an unguarded concurrent map write and it can panic under load. I locked it too since I was already in there.

## Why I deduped this way

I added a `UNIQUE` constraint on `event_id` and switched the insert to `INSERT ... ON CONFLICT (event_id) DO NOTHING`, checking rows affected to tell whether it was a genuine insert or a duplicate. This turns "check if it exists, then insert" into one atomic operation, so there's no window left for two requests to race each other.

I considered using Redis (`SETNX`) as a dedup lock instead it'd be faster since it skips a DB round trip. But it adds a second system that has to stay in sync with Postgres, and if the process crashed between marking something "seen" in Redis and actually writing it to Postgres, you'd silently drop a real call. Since Postgres is already the source of truth here, and the unique constraint gives me that guarantee for free, I went with the simpler option.

## At 10,000 webhooks/sec

I'd add Redis in front of Postgres as a fast pre filter a `SET NX EX` on `event_id` to reject obvious duplicates in microseconds, but keep the Postgres constraint as the real backstop in case Redis loses data, so correctness never depends on Redis alone. I'd also batch the `account_stats` updates instead of one `UPDATE` per event, and move recording processing off a bare goroutine onto an actual job queue so retries and failures are handled on purpose instead of by accident.

If I had more time, I'd also add a proper integration test that fires concurrent duplicate requests (using goroutines) rather than sequential ones, since that's closer to the real race condition than what my current test covers.