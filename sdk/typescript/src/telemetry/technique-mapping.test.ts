/**
 * Tests for the interim attack-class -> Threat Matrix mapping.
 *
 * Guards against fabrication: every mapped technique ID must be a real, valid
 * matrix technique, and unknown classes must never produce a guessed ID.
 */
import { describe, it, expect } from 'vitest';
import {
  INTERIM_ATTACK_CLASS_MAP,
  KNOWN_TECHNIQUE_IDS,
  isValidTechniqueId,
  isTechniqueIdFormat,
  mapAttackClass,
  interimTechniqueFields,
} from './technique-mapping';

describe('snapshot integrity', () => {
  it('contains the 61 canonical techniques', () => {
    expect(KNOWN_TECHNIQUE_IDS.size).toBe(61);
  });

  it('every mapped technique ID is a real, valid technique', () => {
    for (const [cls, entry] of Object.entries(INTERIM_ATTACK_CLASS_MAP)) {
      expect(isValidTechniqueId(entry.techniqueId), `${cls} -> ${entry.techniqueId}`).toBe(true);
      expect(entry.matrixAttackClass).toBeTruthy();
    }
  });
});

describe('isValidTechniqueId', () => {
  it('accepts real IDs', () => {
    expect(isValidTechniqueId('T-2002')).toBe(true);
  });
  it('rejects well-formed but non-existent IDs', () => {
    expect(isValidTechniqueId('T-9999')).toBe(false);
    expect(isTechniqueIdFormat('T-9999')).toBe(true); // format-only is laxer
  });
  it('rejects malformed values', () => {
    expect(isValidTechniqueId('t-2002')).toBe(false);
    expect(isValidTechniqueId('T-22')).toBe(false);
    expect(isValidTechniqueId(undefined)).toBe(false);
  });
});

describe('mapAttackClass', () => {
  it('maps the six injection classes to their representative techniques', () => {
    expect(mapAttackClass('indirect')?.techniqueId).toBe('T-4002');
    expect(mapAttackClass('direct')?.techniqueId).toBe('T-1003');
    expect(mapAttackClass('rag-embedded')?.techniqueId).toBe('T-2002');
    expect(mapAttackClass('multi-turn')?.techniqueId).toBe('T-2004');
    expect(mapAttackClass('tool-output')?.techniqueId).toBe('T-4007');
    expect(mapAttackClass('jailbreak')?.techniqueId).toBe('T-2001');
  });

  it('normalizes case, separators, and aliases', () => {
    expect(mapAttackClass('RAG_Embedded')?.techniqueId).toBe('T-2002');
    expect(mapAttackClass(' Multi Turn ')?.techniqueId).toBe('T-2004');
    expect(mapAttackClass('prompt-injection')?.techniqueId).toBe('T-1003');
    expect(mapAttackClass('indirect-prompt-injection')?.techniqueId).toBe('T-4002');
  });

  it('returns undefined for unknown classes — never guesses', () => {
    expect(mapAttackClass('totally-unknown')).toBeUndefined();
    expect(mapAttackClass('')).toBeUndefined();
    expect(mapAttackClass(undefined)).toBeUndefined();
  });
});

describe('interimTechniqueFields', () => {
  it('always reports the interim source', () => {
    expect(interimTechniqueFields('indirect').techniqueSource).toBe('interim-mapping');
    expect(interimTechniqueFields('unknown').techniqueSource).toBe('interim-mapping');
  });

  it('sets techniqueId only for known classes', () => {
    expect(interimTechniqueFields('rag-embedded').techniqueId).toBe('T-2002');
    expect(interimTechniqueFields('unknown').techniqueId).toBeUndefined();
  });
});
