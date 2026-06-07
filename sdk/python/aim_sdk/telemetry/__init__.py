"""
Causal-denial telemetry.

Captures why a blocked agent action happened by joining the injection cause
(detection inference), the classified intent, and the authorization outcome
(enforcement fact) into one correlated record. The full record is authoritative
and stays local; only an anonymized indicator may be shared.

Everything here is off the enforcement critical path and best-effort.
"""
from .correlation import (
    CORRELATION_ID_PREFIX,
    CORRELATION_HEADER,
    CORRELATION_BAGGAGE_KEY,
    mint_correlation_id,
    is_correlation_id,
    correlation_headers,
    extract_correlation_id,
)
from .correlated_record import (
    TELEMETRY_SCHEMA_VERSION,
    EnforcementInput,
    IntentInput,
    DetectionInput,
    EnforcementFact,
    IntentInference,
    DetectionInference,
    AssemblyMeta,
    CorrelatedRecord,
    SharedIndicator,
    SharedIndicatorContext,
    build_correlated_record,
    assert_observed_invariant,
    to_shared_indicator,
    record_from_dict,
)
from .local_writer import (
    CORRELATED_FILE,
    default_data_dir,
    write_correlated_record,
    read_correlated_records,
)
from .joiner import CorrelationJoiner, is_high_confidence
from .technique_mapping import (
    MATRIX_SNAPSHOT_VERSION,
    TECHNIQUE_ID_RE,
    KNOWN_TECHNIQUE_IDS,
    INTERIM_ATTACK_CLASS_MAP,
    InterimMappingEntry,
    is_valid_technique_id,
    is_technique_id_format,
    map_attack_class,
    interim_technique_fields,
)
from .relay import (
    CorrelatedRelay,
    FlushResult,
    DEFAULT_REGISTRY_URL,
    RUNTIME_TELEMETRY_PATH,
    RELAY_EVENT_TYPE,
    SENTINEL_PACKAGE_NAME,
    open_a2a_home,
    derive_sensor_token,
    detect_runtime_env,
    days_since_install,
)

__all__ = [
    # correlation
    "CORRELATION_ID_PREFIX",
    "CORRELATION_HEADER",
    "CORRELATION_BAGGAGE_KEY",
    "mint_correlation_id",
    "is_correlation_id",
    "correlation_headers",
    "extract_correlation_id",
    # correlated record
    "TELEMETRY_SCHEMA_VERSION",
    "EnforcementInput",
    "IntentInput",
    "DetectionInput",
    "EnforcementFact",
    "IntentInference",
    "DetectionInference",
    "AssemblyMeta",
    "CorrelatedRecord",
    "SharedIndicator",
    "SharedIndicatorContext",
    "build_correlated_record",
    "assert_observed_invariant",
    "to_shared_indicator",
    "record_from_dict",
    # local writer
    "CORRELATED_FILE",
    "default_data_dir",
    "write_correlated_record",
    "read_correlated_records",
    # joiner
    "CorrelationJoiner",
    "is_high_confidence",
    # technique mapping
    "MATRIX_SNAPSHOT_VERSION",
    "TECHNIQUE_ID_RE",
    "KNOWN_TECHNIQUE_IDS",
    "INTERIM_ATTACK_CLASS_MAP",
    "InterimMappingEntry",
    "is_valid_technique_id",
    "is_technique_id_format",
    "map_attack_class",
    "interim_technique_fields",
    # relay
    "CorrelatedRelay",
    "FlushResult",
    "DEFAULT_REGISTRY_URL",
    "RUNTIME_TELEMETRY_PATH",
    "RELAY_EVENT_TYPE",
    "SENTINEL_PACKAGE_NAME",
    "open_a2a_home",
    "derive_sensor_token",
    "detect_runtime_env",
    "days_since_install",
]
