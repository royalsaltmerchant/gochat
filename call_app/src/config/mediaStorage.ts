export const AUDIO_DEVICE_STORAGE_KEY = "call_app:last_selected_audio_input";
export const VIDEO_DEVICE_STORAGE_KEY = "call_app:last_selected_video_input";

export function getStoredDeviceId(key: string): string | null {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
}

export function setStoredDeviceId(key: string, deviceId: string | null): void {
  try {
    if (deviceId) {
      localStorage.setItem(key, deviceId);
    } else {
      localStorage.removeItem(key);
    }
  } catch {
    // Ignore storage failures (private mode, disabled storage, etc.)
  }
}
