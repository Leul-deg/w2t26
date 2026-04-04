// ProtectedRoute guards routes that require authentication.
// Redirects to /login with a `next` query param when the session is absent.
// Shows nothing while session state is loading (avoids flash of login page).

import { Navigate, useLocation } from 'react-router-dom';
import { useAuth } from '../auth/AuthContext';

interface ProtectedRouteProps {
  children: React.ReactNode;
  // Optional: require a specific role. Returns 403 page if the user lacks it.
  requireRole?: string;
  // Optional: require a specific permission. Returns 403 page if the user lacks it.
  requirePermission?: string;
}

export default function ProtectedRoute({
  children,
  requireRole,
  requirePermission,
}: ProtectedRouteProps) {
  const { auth, hasRole, hasPermission } = useAuth();
  const location = useLocation();

  if (auth.status === 'loading') {
    // Session check in progress — render nothing to avoid a layout flash.
    return null;
  }

  if (auth.status === 'unauthenticated') {
    // Preserve the intended destination so the login page can redirect back.
    return (
      <Navigate
        to={`/login?next=${encodeURIComponent(location.pathname + location.search)}`}
        replace
      />
    );
  }

  // Authenticated — check role/permission constraints.
  if (requireRole && !hasRole(requireRole)) {
    return <Navigate to="/unauthorized" replace />;
  }
  if (requirePermission && !hasPermission(requirePermission)) {
    return <Navigate to="/unauthorized" replace />;
  }

  return <>{children}</>;
}
