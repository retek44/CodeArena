import { type User, UserManager, WebStorageStateStore } from "oidc-client-ts";
import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { config } from "../config";

export interface AuthUser {
  name: string;
}

export interface Auth {
  mode: "dev" | "cognito";
  user: AuthUser | null;
  ready: boolean;
  signIn: (devName?: string) => Promise<void>;
  completeSignIn: () => Promise<void>;
  signOut: () => Promise<void>;
  headers: () => Promise<Record<string, string>>;
}

const AuthContext = createContext<Auth | null>(null);

export function useAuth(): Auth {
  const auth = useContext(AuthContext);
  if (!auth) throw new Error("useAuth must be used inside <AuthProvider>");
  return auth;
}

export function AuthProvider({ children, value }: { children: ReactNode; value?: Auth }) {
  if (value) return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
  return config.authMode === "cognito" ? (
    <CognitoAuth>{children}</CognitoAuth>
  ) : (
    <DevAuth>{children}</DevAuth>
  );
}

// --- dev mode -----------------------------------------------------------------------

const DEV_USER_KEY = "codearena.devUser";
export const DEV_NAME_PATTERN = /^[a-z0-9-]{1,32}$/;

/** Local development: the API (AUTH_MODE=dev) trusts the X-Dev-User header. */
function DevAuth({ children }: { children: ReactNode }) {
  const [name, setName] = useState<string | null>(() => localStorage.getItem(DEV_USER_KEY));

  const auth = useMemo<Auth>(
    () => ({
      mode: "dev",
      user: name ? { name } : null,
      ready: true,
      signIn: async (devName) => {
        if (!devName || !DEV_NAME_PATTERN.test(devName)) {
          throw new Error("Use 1-32 lowercase letters, digits or dashes.");
        }
        localStorage.setItem(DEV_USER_KEY, devName);
        setName(devName);
      },
      completeSignIn: async () => {},
      signOut: async () => {
        localStorage.removeItem(DEV_USER_KEY);
        setName(null);
      },
      headers: async (): Promise<Record<string, string>> => (name ? { "X-Dev-User": name } : {}),
    }),
    [name],
  );
  return <AuthContext.Provider value={auth}>{children}</AuthContext.Provider>;
}

// --- Cognito ------------------------------------------------------------------------

/** Authorization code flow with PKCE against the Cognito user pool (ADR-0009). */
function CognitoAuth({ children }: { children: ReactNode }) {
  const manager = useMemo(
    () =>
      new UserManager({
        authority: config.cognito.authority,
        client_id: config.cognito.clientId,
        redirect_uri: `${window.location.origin}/auth/callback`,
        post_logout_redirect_uri: `${window.location.origin}/`,
        response_type: "code",
        scope: "openid email profile",
        userStore: new WebStorageStateStore({ store: window.sessionStorage }),
      }),
    [],
  );
  const [user, setUser] = useState<AuthUser | null>(null);
  const [ready, setReady] = useState(false);

  // Subscribe to the token store (an external system): restore the session on load
  // and follow sign-in / sign-out events.
  useEffect(() => {
    let active = true;
    const update = (u: User | null) => {
      if (active) setUser(toAuthUser(u));
    };
    void manager.getUser().then((u) => {
      update(u);
      if (active) setReady(true);
    });
    manager.events.addUserLoaded(update);
    const onUnloaded = () => update(null);
    manager.events.addUserUnloaded(onUnloaded);
    return () => {
      active = false;
      manager.events.removeUserLoaded(update);
      manager.events.removeUserUnloaded(onUnloaded);
    };
  }, [manager]);

  const auth = useMemo<Auth>(
    () => ({
      mode: "cognito",
      user,
      ready,
      signIn: () => manager.signinRedirect(),
      completeSignIn: async () => {
        // Raises userLoaded, which updates the state above.
        await manager.signinRedirectCallback();
      },
      signOut: async () => {
        await manager.removeUser();
        // Cognito has no end_session_endpoint in its discovery document; use its logout URL.
        const logout = new URL(`https://${config.cognito.domain}/logout`);
        logout.searchParams.set("client_id", config.cognito.clientId);
        logout.searchParams.set("logout_uri", `${window.location.origin}/`);
        window.location.assign(logout.toString());
      },
      headers: async (): Promise<Record<string, string>> => {
        const u = await manager.getUser();
        return u && !u.expired ? { Authorization: `Bearer ${u.access_token}` } : {};
      },
    }),
    [manager, user, ready],
  );
  return <AuthContext.Provider value={auth}>{children}</AuthContext.Provider>;
}

function toAuthUser(u: User | null): AuthUser | null {
  return u && !u.expired ? { name: u.profile.email ?? u.profile.sub } : null;
}
