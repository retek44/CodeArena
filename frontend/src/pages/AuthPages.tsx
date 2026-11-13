import { useEffect, useRef, useState, type FormEvent } from "react";
import { Navigate, useNavigate } from "react-router";
import { useAuth } from "../auth/AuthProvider";
import { ErrorMessage } from "../components/ErrorMessage";

export function LoginPage() {
  const auth = useAuth();
  const navigate = useNavigate();
  const [name, setName] = useState("");
  const [error, setError] = useState<unknown>(null);

  if (auth.user) return <Navigate to="/" replace />;

  if (auth.mode === "cognito") {
    return (
      <section>
        <h1>Sign in</h1>
        <button type="button" className="primary" onClick={() => void auth.signIn()}>
          Continue to sign-in
        </button>
      </section>
    );
  }

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault();
    try {
      await auth.signIn(name.trim());
      void navigate("/");
    } catch (err) {
      setError(err);
    }
  };

  return (
    <section>
      <h1>Sign in (local development)</h1>
      <form onSubmit={(e) => void onSubmit(e)}>
        <label htmlFor="dev-name">User name</label>
        <input id="dev-name" value={name} onChange={(e) => setName(e.target.value)} autoComplete="username" />
        {error != null && <ErrorMessage error={error} />}
        <button type="submit" className="primary">
          Sign in
        </button>
      </form>
    </section>
  );
}

export function AuthCallbackPage() {
  const auth = useAuth();
  const navigate = useNavigate();
  const [error, setError] = useState<unknown>(null);
  const started = useRef(false);

  useEffect(() => {
    // React StrictMode runs effects twice; the authorization code can be used only once.
    if (started.current) return;
    started.current = true;
    auth.completeSignIn().then(
      () => void navigate("/", { replace: true }),
      (err: unknown) => setError(err),
    );
  }, [auth, navigate]);

  return error ? <ErrorMessage error={error} /> : <p role="status">Signing in…</p>;
}

export function NotFoundPage() {
  return (
    <section>
      <h1>Page not found</h1>
    </section>
  );
}
