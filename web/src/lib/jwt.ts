interface TokenPayload {
  sub: string   // internalUserID: "{tenantID}/{username}"
  tid: string   // tenantID
  role: string  // "admin" | "member" | "viewer"
  exp?: number
  iat?: number
}

export function decodeToken(token: string): TokenPayload {
  const part = token.split('.')[1]
  return JSON.parse(atob(part.replace(/-/g, '+').replace(/_/g, '/')))
}

export function usernameFromSub(sub: string): string {
  const slash = sub.indexOf('/')
  return slash >= 0 ? sub.slice(slash + 1) : sub
}
