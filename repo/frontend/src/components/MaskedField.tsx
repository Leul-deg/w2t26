// MaskedField displays a privacy-sensitive value.
//
// Default state: shows "••••••" (matching the server's masking convention).
// Reveal state: shows the decrypted value after a successful step-up check.
//
// The "Reveal" button is only rendered if `canReveal` is true (caller checks
// the user's permission, e.g. "readers:reveal_sensitive"). This ensures the
// button is not rendered in a hidden-but-clickable way for unauthorized users.
//
// The reveal flow calls POST /api/v1/readers/:id/reveal, which:
//   1. Validates the step-up password server-side
//   2. Returns decrypted values (or 501 until Phase 6)
//
// Until Phase 6 (reader domain), the reveal returns 501. The component handles
// this gracefully with a "Not yet available" message.

import { useState } from 'react';
import StepUpModal from './StepUpModal';
import { apiClient, HttpError } from '../api/client';

interface MaskedFieldProps {
  label: string;
  fieldKey: string;
  /** The resource ID used in the reveal API call (e.g. reader ID). */
  resourceId: string;
  /** Endpoint prefix for reveal, e.g. '/readers'. Produces /:id/reveal. */
  revealEndpoint: string;
  /** Whether the current user has permission to reveal this field. */
  canReveal: boolean;
}

const MASK = '••••••';

export default function MaskedField({
  label,
  fieldKey,
  resourceId,
  revealEndpoint,
  canReveal,
}: MaskedFieldProps) {
  const [showModal, setShowModal] = useState(false);
  const [revealedValue, setRevealedValue] = useState<string | null>(null);
  const [revealError, setRevealError] = useState<string | null>(null);
  const [revealing, setRevealing] = useState(false);

  async function handleStepUpSuccess() {
    setShowModal(false);
    setRevealError(null);
    setRevealing(true);

    try {
      // The reveal endpoint validates the step-up and returns decrypted fields.
      // POST /api/v1{revealEndpoint}/:id/reveal
      const res = await apiClient.post<Record<string, string>>(
        `${revealEndpoint}/${resourceId}/reveal`,
        {},  // password was already submitted to /auth/stepup by StepUpModal
      );
      const value = res[fieldKey];
      setRevealedValue(value ?? '(empty)');
    } catch (err) {
      if (err instanceof HttpError && err.status === 501) {
        setRevealError('Field decryption not yet available (Phase 6).');
      } else if (err instanceof HttpError && err.status === 401) {
        setRevealError('Step-up session expired. Please try again.');
      } else {
        setRevealError('Could not retrieve field value.');
      }
    } finally {
      setRevealing(false);
    }
  }

  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: '0.625rem',
        padding: '0.375rem 0',
        fontFamily: 'system-ui, -apple-system, sans-serif',
      }}
    >
      <span
        style={{
          fontSize: '0.75rem',
          fontWeight: 600,
          color: '#6b7280',
          minWidth: '120px',
          textTransform: 'uppercase',
          letterSpacing: '0.04em',
        }}
      >
        {label}
      </span>

      <span
        data-testid={`masked-field-${fieldKey}`}
        style={{
          fontFamily: revealedValue ? 'inherit' : 'monospace',
          fontSize: '0.875rem',
          color: revealedValue ? '#1a1a2e' : '#9ca3af',
          letterSpacing: revealedValue ? undefined : '0.15em',
          minWidth: '80px',
        }}
      >
        {revealing ? '…' : (revealedValue ?? MASK)}
      </span>

      {revealError && (
        <span style={{ fontSize: '0.75rem', color: '#dc2626' }}>{revealError}</span>
      )}

      {/* Only render the reveal button if the user has permission */}
      {canReveal && !revealedValue && !revealing && !revealError && (
        <button
          onClick={() => setShowModal(true)}
          aria-label={`Reveal ${label}`}
          style={{
            padding: '0.1875rem 0.625rem',
            background: '#eff6ff',
            color: '#2563eb',
            border: '1px solid #bfdbfe',
            borderRadius: '4px',
            cursor: 'pointer',
            fontSize: '0.6875rem',
            fontWeight: 600,
          }}
        >
          Reveal
        </button>
      )}

      {revealedValue && (
        <button
          onClick={() => { setRevealedValue(null); setRevealError(null); }}
          aria-label={`Hide ${label}`}
          style={{
            padding: '0.1875rem 0.625rem',
            background: '#f3f4f6',
            color: '#6b7280',
            border: '1px solid #e5e7eb',
            borderRadius: '4px',
            cursor: 'pointer',
            fontSize: '0.6875rem',
          }}
        >
          Hide
        </button>
      )}

      {showModal && (
        <StepUpModal
          title={`Reveal ${label}`}
          description={`Enter your password to view the ${label.toLowerCase()} for this record.`}
          onSuccess={handleStepUpSuccess}
          onCancel={() => setShowModal(false)}
        />
      )}
    </div>
  );
}
