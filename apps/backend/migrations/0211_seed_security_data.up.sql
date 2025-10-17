-- Seed realistic security data for testing and demo purposes

-- Get a sample organization and agent (we'll use the first available ones)
DO $$
DECLARE
    v_org_id UUID;
    v_agent_id_1 UUID;
    v_agent_id_2 UUID;
    v_agent_id_3 UUID;
    v_user_id UUID;
BEGIN
    -- Get first organization
    SELECT id INTO v_org_id FROM organizations LIMIT 1;

    -- Get first three agents if they exist
    SELECT id INTO v_agent_id_1 FROM agents WHERE organization_id = v_org_id ORDER BY created_at LIMIT 1;
    SELECT id INTO v_agent_id_2 FROM agents WHERE organization_id = v_org_id ORDER BY created_at LIMIT 1 OFFSET 1;
    SELECT id INTO v_agent_id_3 FROM agents WHERE organization_id = v_org_id ORDER BY created_at LIMIT 1 OFFSET 2;

    -- Get first user
    SELECT id INTO v_user_id FROM users WHERE organization_id = v_org_id LIMIT 1;

    -- Only proceed if we have at least one agent
    IF v_agent_id_1 IS NOT NULL THEN
        -- Insert Security Threats (8 realistic examples)
        INSERT INTO security_threats (id, organization_id, threat_type, severity, title, description, source, target_type, target_id, is_blocked, created_at) VALUES
        (uuid_generate_v4(), v_org_id, 'unauthorized_access', 'critical', 'Unauthorized Access Attempt Detected', 'Multiple failed authentication attempts from suspicious IP address 203.0.113.45. Agent attempted to access restricted resources without proper credentials.', '203.0.113.45', 'agent', v_agent_id_1, false, NOW() - INTERVAL '2 hours'),
        (uuid_generate_v4(), v_org_id, 'suspicious_activity', 'high', 'Unusual API Usage Pattern', 'Agent exhibiting abnormal API call patterns - 350% increase in requests over last 24 hours. Potential data scraping or automated abuse detected.', COALESCE(v_agent_id_2::TEXT, 'system'), 'agent', COALESCE(v_agent_id_2, v_agent_id_1), false, NOW() - INTERVAL '5 hours'),
        (uuid_generate_v4(), v_org_id, 'brute_force', 'critical', 'Brute Force Attack Detected', 'Detected 47 failed login attempts in 3 minutes from IP 198.51.100.23. Attack has been automatically blocked by our security system.', '198.51.100.23', 'agent', v_agent_id_1, true, NOW() - INTERVAL '1 day'),
        (uuid_generate_v4(), v_org_id, 'credential_leak', 'critical', 'Potential Credential Exposure', 'API key detected in public GitHub repository. Immediate action required to revoke and rotate compromised credentials.', 'github_scanner', 'agent', COALESCE(v_agent_id_3, v_agent_id_1), false, NOW() - INTERVAL '12 hours'),
        (uuid_generate_v4(), v_org_id, 'malicious_agent', 'high', 'Suspicious Agent Behavior', 'Agent attempting to execute unauthorized commands outside declared capabilities. Possible malware or compromised agent detected.', COALESCE(v_agent_id_2::TEXT, 'system'), 'agent', COALESCE(v_agent_id_2, v_agent_id_1), false, NOW() - INTERVAL '30 minutes'),
        (uuid_generate_v4(), v_org_id, 'data_exfiltration', 'critical', 'Abnormal Data Transfer Detected', 'Agent transferred 2.4GB of data to external endpoint not in whitelist. Potential data exfiltration in progress.', COALESCE(v_agent_id_3::TEXT, 'system'), 'agent', COALESCE(v_agent_id_3, v_agent_id_1), false, NOW() - INTERVAL '3 hours'),
        (uuid_generate_v4(), v_org_id, 'unauthorized_access', 'medium', 'Permission Escalation Attempt', 'Agent requested admin-level permissions without proper authorization workflow. Request has been denied and logged.', COALESCE(v_agent_id_2::TEXT, 'system'), 'agent', COALESCE(v_agent_id_2, v_agent_id_1), true, NOW() - INTERVAL '2 days'),
        (uuid_generate_v4(), v_org_id, 'suspicious_activity', 'low', 'Unusual Time Access Pattern', 'Agent active during non-standard hours (2:30 AM - 4:00 AM UTC). No malicious activity detected, but flagged for review.', COALESCE(v_agent_id_1::TEXT, 'system'), 'agent', v_agent_id_1, false, NOW() - INTERVAL '18 hours');

        -- Insert Security Anomalies (5 examples)
        INSERT INTO security_anomalies (id, organization_id, anomaly_type, severity, title, description, resource_type, resource_id, confidence, created_at) VALUES
        (uuid_generate_v4(), v_org_id, 'unusual_api_usage', 'high', 'API Rate Limit Near Threshold', 'Agent consuming 85% of allocated API quota. Current rate: 850 requests/min (limit: 1000/min). May require plan upgrade or optimization.', 'agent', v_agent_id_1, 92.5, NOW() - INTERVAL '1 hour'),
        (uuid_generate_v4(), v_org_id, 'abnormal_traffic', 'medium', 'Traffic Spike Detected', 'Sudden 400% increase in network traffic from agent. No malicious patterns detected, but monitoring closely.', 'agent', COALESCE(v_agent_id_2, v_agent_id_1), 78.3, NOW() - INTERVAL '6 hours'),
        (uuid_generate_v4(), v_org_id, 'unexpected_location', 'low', 'Geographic Location Change', 'Agent accessed from new geographic region (Singapore). Previous access: United States. Legitimate if agent deployed to new region.', 'agent', COALESCE(v_agent_id_3, v_agent_id_1), 65.0, NOW() - INTERVAL '4 hours'),
        (uuid_generate_v4(), v_org_id, 'unusual_access_pattern', 'medium', 'Non-Sequential Resource Access', 'Agent accessing resources in unusual non-sequential pattern. May indicate automated scanning or reconnaissance activity.', 'agent', v_agent_id_1, 71.8, NOW() - INTERVAL '8 hours'),
        (uuid_generate_v4(), v_org_id, 'rate_limit_violation', 'high', 'Rate Limit Exceeded', 'Agent exceeded API rate limit 12 times in last hour. Temporary throttling applied to prevent service degradation.', 'agent', COALESCE(v_agent_id_2, v_agent_id_1), 95.2, NOW() - INTERVAL '2 hours');

        -- Insert Security Incidents (4 examples)
        INSERT INTO security_incidents (id, organization_id, incident_type, status, severity, title, description, affected_resources, assigned_to, created_at, updated_at) VALUES
        (uuid_generate_v4(), v_org_id, 'security_breach', 'investigating', 'critical', 'Suspected Account Takeover', 'Multiple agents accessed from same IP address within short timeframe. Investigating potential account compromise. User credentials may be compromised.', ARRAY[v_agent_id_1::TEXT, COALESCE(v_agent_id_2::TEXT, v_agent_id_1::TEXT)], v_user_id, NOW() - INTERVAL '6 hours', NOW() - INTERVAL '4 hours'),
        (uuid_generate_v4(), v_org_id, 'policy_violation', 'open', 'high', 'Compliance Violation Detected', 'Agent processed PII data without required encryption. Violates data protection policy and GDPR requirements. Immediate remediation required.', ARRAY[COALESCE(v_agent_id_3::TEXT, v_agent_id_1::TEXT)], v_user_id, NOW() - INTERVAL '10 hours', NOW() - INTERVAL '10 hours'),
        (uuid_generate_v4(), v_org_id, 'unauthorized_access', 'resolved', 'medium', 'Privilege Escalation Blocked', 'Agent attempted privilege escalation exploit. Attack successfully blocked by security controls. No data compromise detected.', ARRAY[COALESCE(v_agent_id_2::TEXT, v_agent_id_1::TEXT)], v_user_id, NOW() - INTERVAL '3 days', NOW() - INTERVAL '2 days'),
        (uuid_generate_v4(), v_org_id, 'ddos_attack', 'open', 'critical', 'Distributed Denial of Service Attack', 'Coordinated attack from 15 IP addresses targeting agent endpoints. Attack traffic: 450 requests/second. DDoS mitigation activated.', ARRAY[v_agent_id_1::TEXT], v_user_id, NOW() - INTERVAL '45 minutes', NOW() - INTERVAL '45 minutes');

        -- Insert Security Scan Results (2 examples)
        INSERT INTO security_scans (id, organization_id, scan_type, status, threats_found, anomalies_found, vulnerabilities_found, security_score, started_at, completed_at) VALUES
        (uuid_generate_v4(), v_org_id, 'full', 'completed', 8, 5, 3, 72.5, NOW() - INTERVAL '1 day', NOW() - INTERVAL '1 day' + INTERVAL '15 minutes'),
        (uuid_generate_v4(), v_org_id, 'quick', 'completed', 2, 1, 0, 88.3, NOW() - INTERVAL '6 hours', NOW() - INTERVAL '6 hours' + INTERVAL '2 minutes');
    END IF;
END $$;
