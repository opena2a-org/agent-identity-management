-- Remove seed security data
DELETE FROM security_scans WHERE scan_type IN ('full', 'quick');
DELETE FROM security_incidents WHERE incident_type IN ('security_breach', 'policy_violation', 'unauthorized_access', 'ddos_attack');
DELETE FROM security_anomalies WHERE anomaly_type IN ('unusual_api_usage', 'abnormal_traffic', 'unexpected_location', 'unusual_access_pattern', 'rate_limit_violation');
DELETE FROM security_threats WHERE threat_type IN ('unauthorized_access', 'suspicious_activity', 'brute_force', 'credential_leak', 'malicious_agent', 'data_exfiltration');
