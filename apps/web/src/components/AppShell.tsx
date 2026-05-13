import { Link, NavLink, Outlet, useNavigate } from "react-router-dom";
import { useAuth } from "../contexts/AuthContext";

const navItems = [
  { to: "/app/assessments", label: "Assessments" },
  { to: "/app/team", label: "Team" },
  { to: "/app/connectors", label: "Connectors" },
];

export function AppShell() {
  const { user, logout } = useAuth();
  const navigate = useNavigate();

  function handleLogout() {
    logout();
    navigate("/login");
  }

  return (
    <div className="flex min-h-screen">
      {/* Sidebar */}
      <aside className="flex w-56 flex-col border-r border-surface-border bg-surface-muted">
        <div className="px-4 py-5">
          <Link to="/app" className="flex items-center gap-2">
            <span className="text-xs font-bold uppercase tracking-[0.2em] text-brand-400">
              Readinova
            </span>
          </Link>
        </div>

        <nav className="flex-1 space-y-1 px-2 py-2">
          {navItems.map(({ to, label }) => (
            <NavLink
              key={to}
              to={to}
              className={({ isActive }) =>
                `block rounded-md px-3 py-2 text-sm font-medium transition-colors ${
                  isActive
                    ? "bg-brand-900 text-brand-300"
                    : "text-slate-400 hover:bg-surface hover:text-slate-100"
                }`
              }
            >
              {label}
            </NavLink>
          ))}
        </nav>

        {/* User footer */}
        <div className="border-t border-surface-border px-4 py-4">
          <p className="truncate text-xs text-slate-500">{user?.email}</p>
          <p className="mt-0.5 text-xs capitalize text-slate-600">
            {user?.role}
          </p>
          <button
            onClick={handleLogout}
            className="btn-ghost mt-3 w-full text-xs"
          >
            Sign out
          </button>
        </div>
      </aside>

      {/* Main */}
      <main className="flex-1 overflow-auto p-8">
        <Outlet />
      </main>
    </div>
  );
}
