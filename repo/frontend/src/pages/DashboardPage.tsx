// DashboardPage shows a role-appropriate landing screen for each user type.
// Implemented features are linked; unimplemented domain screens show status only.

import { Link } from 'react-router-dom';
import { useAuth } from '../auth/AuthContext';

const S = {
  heading: {
    fontSize: '1.25rem',
    fontWeight: 700,
    color: '#1a1a2e',
    marginBottom: '0.25rem',
  } as React.CSSProperties,

  subheading: {
    fontSize: '0.875rem',
    color: '#6b7280',
    marginBottom: '1.5rem',
  } as React.CSSProperties,

  grid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fill, minmax(240px, 1fr))',
    gap: '1rem',
    marginBottom: '2rem',
  } as React.CSSProperties,

  card: {
    background: '#fff',
    border: '1px solid #e5e7eb',
    borderRadius: '6px',
    padding: '1.25rem',
  } as React.CSSProperties,

  cardTitle: {
    fontWeight: 600,
    fontSize: '0.875rem',
    marginBottom: '0.25rem',
    color: '#1a1a2e',
  } as React.CSSProperties,

  cardDesc: {
    fontSize: '0.8125rem',
    color: '#6b7280',
    marginBottom: '0.75rem',
  } as React.CSSProperties,

  badge: (impl: boolean): React.CSSProperties => ({
    display: 'inline-block',
    fontSize: '0.6875rem',
    fontWeight: 600,
    padding: '0.125rem 0.5rem',
    borderRadius: '9999px',
    background: impl ? '#dcfce7' : '#fef3c7',
    color: impl ? '#166534' : '#92400e',
  }),

  noticeBox: {
    background: '#eff6ff',
    border: '1px solid #bfdbfe',
    borderRadius: '4px',
    padding: '0.875rem 1rem',
    fontSize: '0.8125rem',
    color: '#1e40af',
    marginBottom: '1.5rem',
  } as React.CSSProperties,
};

interface DomainCard {
  title: string;
  description: string;
  to: string;
  implemented: boolean;
}

const ADMIN_CARDS: DomainCard[] = [
  { title: 'Readers', description: 'Reader profiles, status, and privacy-protected fields', to: '/readers', implemented: false },
  { title: 'Holdings & Copies', description: 'Title catalogue and barcode-level copy management', to: '/holdings', implemented: false },
  { title: 'Circulation', description: 'Checkout, return, and overdue tracking', to: '/circulation', implemented: false },
  { title: 'Stocktake', description: 'Stocktake sessions and variance detection', to: '/stocktake', implemented: false },
  { title: 'Programs', description: 'Scheduled programs with capacity and eligibility rules', to: '/programs', implemented: false },
  { title: 'Enrollments', description: 'Enrollment, waitlists, and add/drop history', to: '/enrollments', implemented: false },
  { title: 'Moderation', description: 'Content review queue and moderation decisions', to: '/moderation', implemented: false },
  { title: 'Feedback & Appeals', description: 'Reader feedback, ratings, and appeal arbitration', to: '/feedback', implemented: false },
  { title: 'Reports', description: 'Configurable reports with branch and role scope', to: '/reports', implemented: true },
  { title: 'Imports/Exports', description: 'Bulk import with validation/rollback; audited exports', to: '/imports', implemented: false },
  { title: 'Users', description: 'Staff accounts, roles, and branch assignments', to: '/users', implemented: false },
];

const OPERATIONS_CARDS: DomainCard[] = [
  { title: 'Readers', description: 'Reader profiles, status, and privacy-protected fields', to: '/readers', implemented: false },
  { title: 'Holdings & Copies', description: 'Title catalogue and barcode-level copy management', to: '/holdings', implemented: false },
  { title: 'Circulation', description: 'Checkout, return, and overdue tracking', to: '/circulation', implemented: false },
  { title: 'Stocktake', description: 'Stocktake sessions and variance detection', to: '/stocktake', implemented: false },
  { title: 'Programs', description: 'Program scheduling and configuration', to: '/programs', implemented: false },
  { title: 'Enrollments', description: 'Enrollment, waitlists, and add/drop history', to: '/enrollments', implemented: false },
  { title: 'Imports/Exports', description: 'Bulk import with validation/rollback; audited exports', to: '/imports', implemented: false },
];

const MODERATOR_CARDS: DomainCard[] = [
  { title: 'Moderation Queue', description: 'Content items pending review and decision', to: '/moderation', implemented: false },
  { title: 'Content', description: 'Governed content items and publishing workflow', to: '/content', implemented: false },
  { title: 'Feedback', description: 'Reader feedback and ratings awaiting moderation', to: '/feedback', implemented: false },
  { title: 'Appeals', description: 'Reader appeals and arbitration outcomes', to: '/appeals', implemented: false },
];

function DomainCardItem({ card }: { card: DomainCard }) {
  return (
    <div style={S.card}>
      <div style={S.cardTitle}>{card.title}</div>
      <div style={S.cardDesc}>{card.description}</div>
      <span style={S.badge(card.implemented)}>
        {card.implemented ? 'Available' : 'Coming soon'}
      </span>
    </div>
  );
}

export default function DashboardPage() {
  const { auth, getPrimaryRole } = useAuth();
  const primaryRole = getPrimaryRole();
  const username = auth.user?.user.username ?? '';

  const roleLabel: Record<string, string> = {
    administrator: 'Administrator',
    operations_staff: 'Operations Staff',
    content_moderator: 'Content Moderator',
  };

  const cards =
    primaryRole === 'administrator'
      ? ADMIN_CARDS
      : primaryRole === 'content_moderator'
        ? MODERATOR_CARDS
        : OPERATIONS_CARDS;

  return (
    <div>
      <div style={S.heading}>Welcome, {username}</div>
      <div style={S.subheading}>
        Signed in as <strong>{roleLabel[primaryRole] ?? primaryRole}</strong>
      </div>

      <div style={S.noticeBox}>
        <strong>Implementation status:</strong> Authentication, session management, RBAC,
        reporting, audited exports, and governed content workflows are implemented.
        Some domain modules remain in progress; the reader reveal flow and several
        operational domains still have deferred pieces.
      </div>

      <div style={{ marginBottom: '1rem', fontWeight: 600, fontSize: '0.875rem' }}>
        Your modules
      </div>

      <div style={S.grid}>
        {cards.map((card) => (
          <DomainCardItem key={card.to} card={card} />
        ))}
      </div>

      <div style={{ fontSize: '0.75rem', color: '#6b7280' }}>
        Backend health:{' '}
        <Link to="/api/v1/health" target="_blank" rel="noreferrer">
          /api/v1/health
        </Link>
        {' · '}
        <Link to="/api/v1/ready" target="_blank" rel="noreferrer">
          /api/v1/ready
        </Link>
      </div>
    </div>
  );
}
