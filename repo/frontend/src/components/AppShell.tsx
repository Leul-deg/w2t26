// AppShell is the authenticated layout: sidebar navigation + top bar + content area.
//
// Responsibilities:
//   - Role-filtered sidebar navigation (users only see allowed routes)
//   - Branch scope display (administrators see "All branches"; others see their scope)
//   - Session inactivity tracking with warning banner and auto-logout
//   - Logout button wired to AuthContext

import { NavLink, Outlet, useNavigate } from 'react-router-dom';
import { useAuth } from '../auth/AuthContext';
import { useSessionTimeout } from '../hooks/useSessionTimeout';
import SessionTimeoutBanner from './SessionTimeoutBanner';

// ── Styles ────────────────────────────────────────────────────────────────────

const S = {
  shell: {
    display: 'flex',
    height: '100vh',
    overflow: 'hidden',
    fontFamily: 'system-ui, -apple-system, sans-serif',
    fontSize: '0.875rem',
    color: '#1a1a2e',
    background: '#f3f4f6',
  } as React.CSSProperties,

  sidebar: {
    width: '220px',
    flexShrink: 0,
    background: '#1e2235',
    color: '#c8d0e0',
    display: 'flex',
    flexDirection: 'column' as const,
    overflow: 'hidden',
  },

  sidebarHeader: {
    padding: '1rem 1rem 0.75rem',
    borderBottom: '1px solid #2a3050',
  } as React.CSSProperties,

  sidebarTitle: {
    fontSize: '0.8125rem',
    fontWeight: 700,
    color: '#e2e8f0',
    letterSpacing: '0.02em',
    lineHeight: 1.3,
  } as React.CSSProperties,

  sidebarNav: {
    flex: 1,
    overflowY: 'auto' as const,
    padding: '0.5rem 0',
  },

  navSection: {
    padding: '0.625rem 1rem 0.25rem',
    fontSize: '0.6875rem',
    fontWeight: 600,
    textTransform: 'uppercase' as const,
    letterSpacing: '0.08em',
    color: '#4a5568',
  } as React.CSSProperties,

  sidebarFooter: {
    borderTop: '1px solid #2a3050',
    padding: '0.75rem 1rem',
  } as React.CSSProperties,

  userBlock: {
    marginBottom: '0.625rem',
  } as React.CSSProperties,

  userName: {
    fontSize: '0.8125rem',
    fontWeight: 600,
    color: '#e2e8f0',
    display: 'block',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap' as const,
  } as React.CSSProperties,

  userMeta: {
    fontSize: '0.6875rem',
    color: '#6b7a99',
    marginTop: '0.125rem',
    display: 'block',
  } as React.CSSProperties,

  branchBadge: {
    display: 'inline-block',
    fontSize: '0.625rem',
    fontWeight: 600,
    padding: '0.125rem 0.5rem',
    borderRadius: '9999px',
    background: '#2a3050',
    color: '#7a8bad',
    marginTop: '0.375rem',
    letterSpacing: '0.04em',
  } as React.CSSProperties,

  logoutBtn: {
    background: 'none',
    border: '1px solid #2a3050',
    color: '#94a3b8',
    padding: '0.375rem 0.75rem',
    borderRadius: '4px',
    cursor: 'pointer',
    fontSize: '0.75rem',
    width: '100%',
    textAlign: 'left' as const,
  },

  main: {
    flex: 1,
    display: 'flex',
    flexDirection: 'column' as const,
    overflow: 'hidden',
    minWidth: 0,
  },

  topBar: {
    background: '#ffffff',
    borderBottom: '1px solid #e5e7eb',
    padding: '0.625rem 1.5rem',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    flexShrink: 0,
  } as React.CSSProperties,

  content: {
    flex: 1,
    overflowY: 'auto' as const,
    padding: '1.5rem',
  },
};

// ── Navigation definitions ────────────────────────────────────────────────────

interface NavItem {
  to: string;
  label: string;
  section?: string;
  roles: string[];
}

const NAV_ITEMS: NavItem[] = [
  { to: '/dashboard',   label: 'Dashboard',         roles: ['administrator', 'operations_staff', 'content_moderator'] },
  { to: '/readers',     label: 'Readers',            section: 'Operations', roles: ['administrator', 'operations_staff', 'content_moderator'] },
  { to: '/holdings',    label: 'Holdings & Copies',  roles: ['administrator', 'operations_staff'] },
  { to: '/circulation', label: 'Circulation',        roles: ['administrator', 'operations_staff'] },
  { to: '/stocktake',   label: 'Stocktake',          roles: ['administrator', 'operations_staff'] },
  { to: '/programs',    label: 'Programs',           section: 'Programs',   roles: ['administrator', 'operations_staff'] },
  { to: '/enrollments', label: 'Enrollments',        roles: ['administrator', 'operations_staff'] },
  { to: '/moderation',  label: 'Moderation Queue',   section: 'Content',    roles: ['administrator', 'content_moderator'] },
  { to: '/content',     label: 'Content',            roles: ['administrator', 'content_moderator'] },
  { to: '/feedback',    label: 'Feedback',           roles: ['administrator', 'operations_staff', 'content_moderator'] },
  { to: '/appeals',     label: 'Appeals',            roles: ['administrator', 'operations_staff', 'content_moderator'] },
  { to: '/reports',     label: 'Reports',            section: 'System',     roles: ['administrator', 'operations_staff', 'content_moderator'] },
  { to: '/imports',     label: 'Imports/Exports',    roles: ['administrator', 'operations_staff'] },
  { to: '/users',       label: 'Users',              roles: ['administrator'] },
];

const ROLE_LABELS: Record<string, string> = {
  administrator: 'Administrator',
  operations_staff: 'Operations Staff',
  content_moderator: 'Content Moderator',
};

function getBranchLabel(roles: string[]): string {
  if (roles.includes('administrator')) return 'All branches';
  // Branch assignment is surfaced in Prompt 6 when the users/branches endpoint exists.
  return 'Assigned branch';
}

// ── NavLink ───────────────────────────────────────────────────────────────────

function SideNavLink({ to, label }: { to: string; label: string }) {
  return (
    <NavLink
      to={to}
      style={({ isActive }) => ({
        display: 'block',
        padding: '0.375rem 1rem',
        color: isActive ? '#e2e8f0' : '#94a3b8',
        background: isActive ? '#2a3a6e' : 'transparent',
        textDecoration: 'none',
        borderLeft: isActive ? '3px solid #4a7fe0' : '3px solid transparent',
        fontSize: '0.8125rem',
        lineHeight: 1.5,
      })}
    >
      {label}
    </NavLink>
  );
}

// ── AppShell ──────────────────────────────────────────────────────────────────

export default function AppShell() {
  const { auth, logout, forceLogout, getPrimaryRole } = useAuth();
  const navigate = useNavigate();

  const primaryRole = getPrimaryRole();
  const username = auth.user?.user.username ?? '';
  const roleLabel = ROLE_LABELS[primaryRole] ?? primaryRole;
  const branchLabel = getBranchLabel(auth.user?.roles ?? []);

  // Session inactivity timeout — warn at 25 min, force-logout at 30 min.
  // These match the server's SESSION_INACTIVITY_SECONDS = 1800 (30 min).
  const { isExpiringSoon, extendSession } = useSessionTimeout({
    isActive: auth.status === 'authenticated',
    onWarn: () => {/* banner shows via isExpiringSoon */},
    onExpire: () => {
      forceLogout();
      navigate('/login?reason=expired', { replace: true });
    },
  });

  // Filter nav items to those the current user's roles permit.
  const visibleItems = NAV_ITEMS.filter((item) =>
    auth.user?.roles.some((r) => item.roles.includes(r)),
  );

  async function handleLogout() {
    await logout();
    navigate('/login', { replace: true });
  }

  function handleExtendSession() {
    extendSession();
  }

  // Track last rendered section header to avoid duplicates.
  let lastSection: string | undefined;

  return (
    <div style={S.shell}>
      {/* ── Sidebar ── */}
      <aside style={S.sidebar}>
        <div style={S.sidebarHeader}>
          <div style={S.sidebarTitle}>
            Library Operations<br />
            &amp; Enrollment Suite
          </div>
        </div>

        <nav style={S.sidebarNav} aria-label="Main navigation">
          {visibleItems.map((item) => {
            const showSection = item.section && item.section !== lastSection;
            if (showSection) lastSection = item.section;
            return (
              <div key={item.to}>
                {showSection && (
                  <div style={S.navSection} aria-hidden="true">
                    {item.section}
                  </div>
                )}
                <SideNavLink to={item.to} label={item.label} />
              </div>
            );
          })}
        </nav>

        <div style={S.sidebarFooter}>
          <div style={S.userBlock}>
            <span style={S.userName}>{username}</span>
            <span style={S.userMeta}>{roleLabel}</span>
            <span style={S.branchBadge}>{branchLabel}</span>
          </div>
          <button style={S.logoutBtn} onClick={handleLogout} type="button">
            Sign out
          </button>
        </div>
      </aside>

      {/* ── Main content ── */}
      <div style={S.main}>
        <header style={S.topBar}>
          <span style={{ fontWeight: 600, fontSize: '0.875rem', color: '#1a1a2e' }}>
            Library Management System
          </span>
          <span style={{ fontSize: '0.75rem', color: '#6b7280' }}>
            <strong>{username}</strong> · {roleLabel} · {branchLabel}
          </span>
        </header>

        <main style={S.content}>
          <Outlet />
        </main>
      </div>

      {/* ── Session expiry warning ── */}
      {isExpiringSoon && (
        <SessionTimeoutBanner
          onExtend={handleExtendSession}
          onSignOut={handleLogout}
        />
      )}
    </div>
  );
}
