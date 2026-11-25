# zero-click-refresh

## 1. Roster portraits fetch themselves

- [ ] 1.1 `internal/ui/views/analysis/analysis_avatars.go`: add `ensureAvatarsAsync()` that
      returns early when `avatarAutoFetch` is false, no players, avatar busy, or every player
      already has a cached portrait.
- [ ] 1.2 Fetch only missing portraits with `avatars.DefaultMaxAge` through
      `View.avatarClient` (nil → `avatars.NewClient()`), reload bitmaps and invalidate.
- [ ] 1.3 `analysis.go`: default `avatarAutoFetch: true` in `NewView`, call
      `ensureAvatarsAsync()` from `SelectDemo`.
- [ ] 1.4 Tests: `TestEnsureAvatarsAsyncFetchesMissing` (httptest profile XML + PNG,
      asserts request steamid and decoded bitmap), `TestEnsureAvatarsAsyncRespectsDisabled`;
      render tests set `avatarAutoFetch = false`.

## 2. Ban snapshots refresh themselves

- [ ] 2.1 `internal/ui/views/library/bans_refresh.go`: add `ensureBansAsync()` guarded by an
      API key, `bansBusy`, and `refreshBans(..., refresh=false)`.
- [ ] 2.2 `library.go`: call it at the end of `Refresh`, add the `bansBusy` field.
- [ ] 2.3 Test: `TestEnsureBansAsyncWithoutKeyIsNoop`.

## 3. Verification

- [ ] 3.1 `go build ./...`, `go vet ./...`, `gofmt -l cmd internal` clean.
- [ ] 3.2 `go test ./... -count=1` green.
- [ ] 3.3 `openspec validate zero-click-refresh --strict`, then archive.
