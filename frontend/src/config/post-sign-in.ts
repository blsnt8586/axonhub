export function getSafeInternalRedirect(value: string | undefined, origin: string) {
  if (!value || !value.startsWith('/') || value.startsWith('//')) {
    return undefined;
  }

  try {
    const url = new URL(value, origin);
    return url.origin === origin ? `${url.pathname}${url.search}${url.hash}` : undefined;
  } catch {
    return undefined;
  }
}
