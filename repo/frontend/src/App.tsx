// App is the root component. It wires together:
//   - AuthProvider (session state, login/logout operations)
//   - BrowserRouter (client-side routing)
//   - Route table (protected routes filtered by role)

import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom';
import { AuthProvider } from './auth/AuthContext';
import AppShell from './components/AppShell';
import ProtectedRoute from './components/ProtectedRoute';
import LoginPage from './pages/LoginPage';
import DashboardPage from './pages/DashboardPage';
import UnauthorizedPage from './pages/UnauthorizedPage';
import NotFoundPage from './pages/NotFoundPage';
import ReadersListPage from './pages/readers/ReadersListPage';
import ReaderDetailPage from './pages/readers/ReaderDetailPage';
import ReaderFormPage from './pages/readers/ReaderFormPage';
import ImportPage from './pages/imports/ImportPage';
import ImportHistoryPage from './pages/imports/ImportHistoryPage';
import ExportsPage from './pages/exports/ExportsPage';
import ProgramsListPage from './pages/programs/ProgramsListPage';
import ProgramDetailPage from './pages/programs/ProgramDetailPage';
import ProgramFormPage from './pages/programs/ProgramFormPage';
import ContentListPage from './pages/content/ContentListPage';
import ContentFormPage from './pages/content/ContentFormPage';
import ModerationQueuePage from './pages/moderation/ModerationQueuePage';
import ContentReviewPage from './pages/moderation/ContentReviewPage';
import FeedbackPage from './pages/feedback/FeedbackPage';
import AppealsListPage from './pages/appeals/AppealsListPage';
import AppealDetailPage from './pages/appeals/AppealDetailPage';
import ReportsPage from './pages/reports/ReportsPage';
import HoldingsListPage from './pages/holdings/HoldingsListPage';
import HoldingDetailPage from './pages/holdings/HoldingDetailPage';
import StocktakePage from './pages/stocktake/StocktakePage';
import CirculationPage from './pages/circulation/CirculationPage';
import UsersPage from './pages/users/UsersPage';
import EnrollmentsPage from './pages/enrollments/EnrollmentsPage';

export default function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <Routes>
          {/* Public routes */}
          <Route path="/login" element={<LoginPage />} />
          <Route path="/unauthorized" element={<UnauthorizedPage />} />

          {/* Protected routes — wrapped by AppShell (sidebar + topbar layout) */}
          <Route
            element={
              <ProtectedRoute>
                <AppShell />
              </ProtectedRoute>
            }
          >
            {/* Root redirect */}
            <Route index element={<Navigate to="/dashboard" replace />} />

            {/* Dashboard */}
            <Route path="/dashboard" element={<DashboardPage />} />

            {/* Readers — fully implemented */}
            <Route path="/readers" element={<ReadersListPage />} />
            <Route path="/readers/new" element={<ReaderFormPage />} />
            <Route path="/readers/:id" element={<ReaderDetailPage />} />
            <Route path="/readers/:id/edit" element={<ReaderFormPage />} />

            {/* Imports — upload, preview, commit flow */}
            <Route path="/imports" element={<ImportPage />} />
            <Route path="/imports/history" element={<ImportHistoryPage />} />

            {/* Exports — generate and audit log */}
            <Route path="/exports" element={<ExportsPage />} />

            {/* Programs — list, detail, create/edit */}
            <Route path="/programs" element={<ProgramsListPage />} />
            <Route path="/programs/new" element={<ProgramFormPage />} />
            <Route path="/programs/:id" element={<ProgramDetailPage />} />
            <Route path="/programs/:id/edit" element={<ProgramFormPage />} />

            {/* Content governance */}
            <Route path="/content" element={<ContentListPage />} />
            <Route path="/content/new" element={<ContentFormPage />} />
            <Route path="/content/:id" element={<ContentFormPage />} />

            {/* Moderation queue */}
            <Route path="/moderation" element={<ModerationQueuePage />} />
            <Route path="/moderation/:id" element={<ContentReviewPage />} />

            {/* Feedback */}
            <Route path="/feedback" element={<FeedbackPage />} />

            {/* Appeals */}
            <Route path="/appeals" element={<AppealsListPage />} />
            <Route path="/appeals/:id" element={<AppealDetailPage />} />

            {/* Reports */}
            <Route path="/reports" element={<ReportsPage />} />

            {/* Holdings — fully implemented */}
            <Route path="/holdings" element={<HoldingsListPage />} />
            <Route path="/holdings/:id" element={<HoldingDetailPage />} />

            {/* Stocktake — fully implemented */}
            <Route path="/stocktake" element={<StocktakePage />} />

            {/* Circulation — fully implemented */}
            <Route path="/circulation" element={<CirculationPage />} />

            {/* Enrollments */}
            <Route path="/enrollments" element={<EnrollmentsPage />} />

            {/* User management */}
            <Route path="/users" element={<UsersPage />} />
          </Route>

          {/* 404 catch-all */}
          <Route path="*" element={<NotFoundPage />} />
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  );
}
