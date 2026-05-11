/**
 * notifications.ts — Web Push subscription registration.
 *
 * Call registerPushSubscription() after the user grants notification
 * permission. The VAPID public key is read from the VITE_VAPID_PUBLIC_KEY env
 * variable; the server provides it at build time.
 *
 * TODO: load VAPID public key from server config endpoint rather than env var.
 */

// VAPID public key (base64url). Set VITE_VAPID_PUBLIC_KEY at build time or
// provide it via server config. Empty string disables push registration.
const VAPID_PUBLIC_KEY = import.meta.env.VITE_VAPID_PUBLIC_KEY ?? "";

function urlBase64ToUint8Array(base64String: string): Uint8Array<ArrayBuffer> {
  const padding = "=".repeat((4 - (base64String.length % 4)) % 4);
  const base64 = (base64String + padding).replace(/-/g, "+").replace(/_/g, "/");
  const raw = atob(base64);
  return new Uint8Array([...raw].map((c) => c.charCodeAt(0)));
}

/**
 * Register a Web Push subscription and POST it to the server-side PushService.
 * Safe to call multiple times — the push manager de-duplicates subscriptions.
 */
export async function registerPushSubscription(): Promise<void> {
  if (!("serviceWorker" in navigator) || !("PushManager" in window)) return;
  if (!VAPID_PUBLIC_KEY) return;

  const permission = await Notification.requestPermission();
  if (permission !== "granted") return;

  const reg = await navigator.serviceWorker.ready;
  const sub = await reg.pushManager.subscribe({
    userVisibleOnly: true,
    applicationServerKey: urlBase64ToUint8Array(VAPID_PUBLIC_KEY),
  });

  // POST the subscription to the server-side PushService
  await fetch("/api/push/subscribe", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      endpoint: sub.endpoint,
      p256dh: btoa(String.fromCharCode(...new Uint8Array(sub.getKey("p256dh")!))),
      auth: btoa(String.fromCharCode(...new Uint8Array(sub.getKey("auth")!))),
      userAgent: navigator.userAgent,
    }),
  });
}

/**
 * Unregister the current push subscription and notify the server.
 */
export async function unregisterPushSubscription(): Promise<void> {
  if (!("serviceWorker" in navigator)) return;

  const reg = await navigator.serviceWorker.ready;
  const sub = await reg.pushManager.getSubscription();
  if (!sub) return;

  await fetch("/api/push/unsubscribe", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ endpoint: sub.endpoint }),
  });

  await sub.unsubscribe();
}
