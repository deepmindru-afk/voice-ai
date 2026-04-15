import { Metadata } from '@rapidaai/react';
import { getOptionValue, buildDefaultMetadata } from '../common';

// ============================================================================
// Constants
// ============================================================================

const REQUIRED_KEYS = ['tool.transfer_to'];

// ============================================================================
// Default Options
// ============================================================================

export const GetTransferCallDefaultOptions = (
  current: Metadata[],
): Metadata[] =>
  buildDefaultMetadata(
    current,
    [{ key: 'tool.transfer_to' }],
    REQUIRED_KEYS,
  );

// ============================================================================
// Validation
// ============================================================================

export const ValidateTransferCallDefaultOptions = (
  options: Metadata[],
): string | undefined => {
  const transferTo = getOptionValue(options, 'tool.transfer_to');
  if (!transferTo || !transferTo.trim()) {
    return 'Please provide a phone number or SIP URI to transfer calls to.';
  }
  return undefined;
};
