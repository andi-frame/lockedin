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
