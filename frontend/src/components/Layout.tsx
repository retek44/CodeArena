import { Link, NavLink, Outlet } from "react-router";
import { useAuth } from "../auth/AuthProvider";

export function Layout() {
  const auth = useAuth();
  return (
    <>
      <a className="skip-link" href="#main">
        Skip to content
      </a>
      <header className="header">
        <Link to="/" className="brand">
          CodeArena
        </Link>
        <nav aria-label="Main">
          <NavLink to="/">Problems</NavLink>
          {auth.user && <NavLink to="/submissions">My submissions</NavLink>}
        </nav>
        <div className="user">
          {auth.user ? (
            <>
              <span>{auth.user.name}</span>
              <button type="button" onClick={() => void auth.signOut()}>
                Sign out
              </button>
            </>
          ) : (
            <Link to="/login">Sign in</Link>
          )}
        </div>
      </header>
      <main id="main" className="main">
        <Outlet />
      </main>
    </>
  );
}
