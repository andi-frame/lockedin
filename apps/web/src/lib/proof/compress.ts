import { targetSize } from "./net";

const MAX_SIDE = 2048; // what the server stores (SPEC §8), so a larger picture is wasted upload time
const QUALITY = 0.8;

// Formats the browser cannot reliably draw are sent as they are; the server converts them.
const skip = new Set(["image/gif", "image/heic", "image/heif", "image/avif"]);

/**
 * Shrinks a photo in the browser before upload: longest side at most 2048 px, WebP at 80%. If
 * anything about that fails, or the result is not smaller, the original goes up unchanged. The
 * server re-encodes and strips EXIF either way, so this only saves time on a slow connection.
 */
export async function compressImage(file: File): Promise<File> {
  if (!file.type.startsWith("image/") || skip.has(file.type)) return file;
  try {
    const bitmap = await createImageBitmap(file, { imageOrientation: "from-image" });
    const { width, height } = targetSize(bitmap.width, bitmap.height, MAX_SIDE);
    const canvas = document.createElement("canvas");
    canvas.width = width;
    canvas.height = height;
    canvas.getContext("2d")?.drawImage(bitmap, 0, 0, width, height);
    bitmap.close();
    const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, "image/webp", QUALITY));
    if (!blob || blob.type !== "image/webp" || blob.size >= file.size) return file;
    return new File([blob], file.name.replace(/\.[^.]+$/, "") + ".webp", { type: "image/webp", lastModified: file.lastModified });
  } catch {
    return file;
  }
}
