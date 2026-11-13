export type AuthMode = "dev" | "cognito";

function authMode(): AuthMode {
  const mode = import.meta.env.VITE_AUTH_MODE ?? "dev";
  if (mode !== "dev" && mode !== "cognito") {
    throw new Error(`VITE_AUTH_MODE must be dev or cognito, got ${String(mode)}`);
  }
  return mode;
}

export const config = {
  authMode: authMode(),
  cognito: {
    authority: import.meta.env.VITE_COGNITO_AUTHORITY ?? "",
    clientId: import.meta.env.VITE_COGNITO_CLIENT_ID ?? "",
    domain: import.meta.env.VITE_COGNITO_DOMAIN ?? "",
  },
};
