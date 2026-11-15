import { Link, NavLink, Outlet, useLocation } from "react-router";
import { useMe } from "../api/hooks";
import { useAuth } from "../auth/AuthProvider";

const BRAND = "CodeArena";

export function Layout() {
  const auth = useAuth();
  const me = useMe();
  // A problem page belongs to the Problems section too.
  const { pathname } = useLocation();
  const onProblems = pathname === "/" || pathname.startsWith("/problems/");
  return (
    <>
      <a className="skip-link" href="#main">
        Skip to content
      </a>
      <header className="header">
        {/* The name set in flap cells, one letter each; the link's accessible name stays "CodeArena". */}
        <Link to="/" className="brand" aria-label={BRAND}>
          {[...BRAND].map((letter, i) => (
            <span key={i} aria-hidden="true">
              {letter}
            </span>
          ))}
        </Link>
        <nav aria-label="Main">
          <NavLink
            to="/"
            className={onProblems ? "active" : undefined}
            aria-current={onProblems ? "page" : undefined}
          >
            Problems
          </NavLink>
          <NavLink to="/contests">Contests</NavLink>
          {auth.user && <NavLink to="/submissions">My submissions</NavLink>}
          {auth.user && me.data?.admin && <NavLink to="/admin/problems">Admin</NavLink>}
        </nav>
        <div className="user">
          {auth.user ? (
            <>
              <span className="user-name">{auth.user.name}</span>
              <button type="button" className="quiet" onClick={() => void auth.signOut()}>
                Sign out
              </button>
            </>
          ) : (
            <Link to="/login" className="button-link">
              Sign in
            </Link>
          )}
        </div>
      </header>
      <main id="main" className="app-main">
        <Outlet />
      </main>
    </>
  );
}
