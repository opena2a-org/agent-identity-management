"""
The tags passed to ``secure()`` for an agent that already exists reach the
backend in the form its tag route reads.

``POST /api/v1/agents/{id}/tags`` reads tag IDs only. Its handler
(``AddTagsToAgent`` in ``apps/backend/internal/interfaces/http/handlers/tag_handler.go``)
parses every ``tagIds`` entry as a UUID, answers 400 ``Invalid tag ID format``
for the whole request when one entry is not, and answers 204 with no body when
the tags are added. A tag name is resolved by the backend only when the agent
is first registered.

The SDK sent every requested entry to that route, so one tag name made the
route refuse the request, tag IDs in the same list included, and it read the
204 as a failure.
"""

import json

import pytest
import responses

from aim_sdk import client as client_module
from aim_sdk.client import _sync_agent_tags

AIM_URL = "http://aim.test"
AGENT_ID = "550e8400-e29b-41d4-a716-446655440000"
TAGS_URL = f"{AIM_URL}/api/v1/agents/{AGENT_ID}/tags"
HEADERS = {"Authorization": "Bearer test-token-not-real"}

TAG_ID = "7c9e6679-7425-40de-944b-e07fc1f90ae7"
OTHER_TAG_ID = "f47ac10b-58cc-4372-a567-0e02b2c3d479"


@pytest.fixture
def said(monkeypatch):
    """Record what the SDK reports as a success and as a warning."""
    lines = {"success": [], "warning": []}
    monkeypatch.setattr(client_module.console, "success", lines["success"].append)
    monkeypatch.setattr(client_module.console, "warning", lines["warning"].append)
    return lines


def _tag(tag_id, key):
    """One entry of the route's GET answer, with the members the SDK reads."""
    return {"id": tag_id, "key": key, "value": key, "category": "custom"}


def _sync(tags, on_agent=(), post=None):
    """
    Run the sync against a mocked tag route and return the requests it made.

    ``on_agent`` is the GET answer. ``post`` is the POST answer as keyword
    arguments for ``responses``; with ``post=None`` no POST is registered, so a
    POST the SDK made is still recorded but fails to connect.
    """
    with responses.RequestsMock(assert_all_requests_are_fired=False) as rsps:
        rsps.add(responses.GET, TAGS_URL, json=list(on_agent), status=200)
        if post is not None:
            rsps.add(responses.POST, TAGS_URL, **post)
        _sync_agent_tags(AIM_URL, HEADERS, AGENT_ID, tags)
        return [call.request for call in rsps.calls]


def _posts(requests_made):
    return [r for r in requests_made if r.method == "POST"]


def test_a_tag_name_is_not_sent_and_a_tag_id_beside_it_is(said):
    made = _sync(["production", TAG_ID], post={"status": 204})

    posts = _posts(made)
    assert len(posts) == 1
    assert json.loads(posts[0].body) == {"tagIds": [TAG_ID]}

    assert said["success"] == [f"Applied 1 tag(s): {TAG_ID}"]
    assert len(said["warning"]) == 1
    warning = said["warning"][0]
    assert "production" in warning
    assert TAG_ID not in warning
    assert "first registered" in warning


def test_a_204_answer_is_reported_as_applied(said):
    made = _sync([TAG_ID], post={"status": 204})

    assert len(_posts(made)) == 1
    assert said["success"] == [f"Applied 1 tag(s): {TAG_ID}"]
    assert said["warning"] == []


def test_tag_ids_are_sent_in_the_order_given(said):
    made = _sync([OTHER_TAG_ID, TAG_ID], post={"status": 204})

    assert json.loads(_posts(made)[0].body) == {"tagIds": [OTHER_TAG_ID, TAG_ID]}
    assert said["success"] == [f"Applied 2 tag(s): {OTHER_TAG_ID}, {TAG_ID}"]
    assert said["warning"] == []


def test_tag_names_alone_send_no_request(said):
    made = _sync(["production", "customer-facing"])

    assert [r.method for r in made] == ["GET"]
    assert said["success"] == []
    assert len(said["warning"]) == 1
    assert "production, customer-facing" in said["warning"][0]
    assert "first registered" in said["warning"][0]


def test_a_name_and_an_id_already_on_the_agent_send_no_request(said):
    made = _sync(
        ["production", TAG_ID.upper()],
        on_agent=[_tag(TAG_ID, "production")],
    )

    assert [r.method for r in made] == ["GET"]
    assert said == {"success": [], "warning": []}


def test_a_tag_id_is_sent_once_and_in_the_form_the_backend_returns(said):
    made = _sync([TAG_ID.upper(), TAG_ID.replace("-", ""), TAG_ID], post={"status": 204})

    assert json.loads(_posts(made)[0].body) == {"tagIds": [TAG_ID]}
    assert said["success"] == [f"Applied 1 tag(s): {TAG_ID}"]
    assert said["warning"] == []


@pytest.mark.parametrize(
    "entry",
    [
        # 32 characters that Python's uuid.UUID() reads and the backend's parser refuses.
        "0x" + "a" * 30,
        # The digits of a UUID with one hyphen missing.
        "7c9e66797425-40de-944b-e07fc1f90ae7",
    ],
    ids=["hex-prefix", "misplaced-hyphens"],
)
def test_an_entry_the_backend_would_not_read_as_a_uuid_is_not_sent(said, entry):
    made = _sync([entry, TAG_ID], post={"status": 204})

    assert json.loads(_posts(made)[0].body) == {"tagIds": [TAG_ID]}
    assert len(said["warning"]) == 1
    assert entry in said["warning"][0]


def test_a_refused_request_is_reported_with_the_backend_error(said):
    made = _sync([TAG_ID], post={"status": 401, "json": {"error": "Unauthorized"}})

    assert len(_posts(made)) == 1
    assert said["success"] == []
    assert said["warning"] == ["Failed to apply tags: Unauthorized"]
