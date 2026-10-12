"""Links to dashboard pages.

The SDK talks to the AIM server at ``aim_url``. The dashboard is a separate
web app, on another origin wherever the two are deployed apart: the hosted
dashboard does not share its host with the hosted API, and a local stack
serves the API on port 8080 and the dashboard on port 3000. The server
registers no /dashboard route, so a dashboard link is built on the URL
``dashboard_url_for`` returns, never on ``aim_url``.
"""

# The dashboard page where an administrator approves or denies a pending JIT
# verification (the dashboard's "JIT requests" tab).
JIT_APPROVALS_PAGE = "/dashboard/admin/jit-requests"


def dashboard_url_for(aim_url: str) -> str:
    """Return the dashboard URL for the AIM server at ``aim_url``.

    The hosted API hosts map to their dashboards, port 8080 maps to port 3000,
    and a trailing ``/api`` path is dropped. Any other URL is returned as it is,
    for a deployment that serves the dashboard and the API on one origin.
    """
    base = aim_url.rstrip("/")
    if "api.aim.opena2a.org" in base:
        return "https://aim.opena2a.org"
    if "api.community.opena2a.org" in base:
        return "https://community.opena2a.org"
    if ":8080" in base:
        return base.replace(":8080", ":3000")
    if base.endswith("/api"):
        return base[: -len("/api")]
    return base
