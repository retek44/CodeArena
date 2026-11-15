import type { ReactNode } from "react";
import { Link } from "react-router";
import { useMe } from "../api/hooks";
import { useAuth } from "../auth/AuthProvider";
import { ErrorMessage } from "./ErrorMessage";

/** Renders `children` for admins only. The API checks it again on every request. */
export function RequireAdmin({ children }: { children: ReactNode }) {
  const auth = useAuth();
  const me = useMe();

  if (!auth.user) {
    return (
      <p>
        <Link to="/login">Sign in</Link> as an admin to manage problems.
      </p>
    );
  }
  if (me.isPending) return <p role="status">Loading…</p>;
  if (me.isError) return <ErrorMessage error={me.error} />;
  if (!me.data.admin) return <p className="error">Only admins can manage problems.</p>;
  return children;
}
