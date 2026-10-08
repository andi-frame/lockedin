# ADR-0005: Garage object storage, presigned uploads, server-side compression

Status: Accepted (2026-10-07)

## Context
Proof often includes photos and videos from phones, and a raw phone video can be over 100 MB. We need to:
- bound storage cost
- keep large request bodies away from the API
- strip location metadata

## Decision
- **Storage:** Garage, which is S3-compatible, lightweight, and self-hosted. It has two buckets:
  - `staging` for raw uploads, cleaned up after 24 h
  - `media` for processed files, private
- **Uploads:** the browser uploads straight to Garage with a presigned PUT. The signature covers `Content-Length` and `Content-Type`. If that can't be used, `UPLOAD_MODE=proxy` streams the upload through the API behind a hard `LimitReader`.
- **Processing:** the worker:
  - sniffs the real file type
  - rejects anything over the limits
  - compresses images to WebP (vips) and videos to 720p H.264 (ffmpeg)
  - strips EXIF/GPS
  - writes the result to `media`
- **Native dev:** a `BlobStore` interface has an `fs` driver, so native dev works without Garage.

## Consequences
- The worker image is built on Debian slim and bundles ffmpeg and libvips. Native dev needs both tools on `PATH`.
- Media URLs are short-lived signed GETs. Nothing is public.
- Limits are configured through env vars (see `docs/SPEC.md §8`).

## Verification: Garage enforces the signed upload (2026-10-07)
`TestPresignedPutRejectsWrongLength` (in the shared contract suite, `internal/storage/contract_test.go`, run against Garage by `s3_integration_test.go`) presigns a PUT for N bytes and then sends N+8 and N-5 bytes. Garage rejects both with `403` and stores nothing. The same suite shows it also rejects a different `Content-Type` and an expired URL. To check the test is not vacuous, the presign was temporarily changed to omit `ContentLength`: the shorter body was then accepted (`200`) and the test failed.

So the signature does pin the length. **`UPLOAD_MODE=presigned` stays the default** and the proxy mode is not implemented. If a deployment ever puts something in front of Garage that rewrites or drops those headers, run this test against it first; only then build the proxy endpoint (`/uploads/{id}/body` behind `io.LimitReader(max+1)`).

Deploys (2026-10-08, task 7.2): the browser reaches Garage on its own host, `media.<DOMAIN>`, through Caddy, not under a `/s3` path prefix. A presigned URL signs its path, so a prefix that the proxy strips before Garage would fail every signature check. The bucket CORS rule is set from inside the compose network (`tepatictl storage-init`), because a deploy does not publish Garage's S3 port.

Driver notes: the s3 client uses path-style addressing and signs presigned URLs for `S3_PUBLIC_ENDPOINT`, a separate client from the one the server uses (`S3_ENDPOINT`). The AWS SDK's default request checksums are switched off (`WhenRequired`).
