"""Modules that each perform one real ``from <module> import *``.

The star-import tests read the names a star import binds from these modules'
namespaces, so the import runs as ordinary module code instead of from a
string of source compiled at test time.
"""

import importlib

# Names the import system sets on every module, whatever the module imports.
MODULE_ATTRS = frozenset(
    {"__name__", "__doc__", "__package__", "__loader__", "__spec__", "__file__", "__cached__", "__builtins__"}
)


def star_namespace(probe: str) -> set:
    """The names the star import in ``star_imports/<probe>.py`` bound."""
    return set(vars(importlib.import_module(f"{__name__}.{probe}"))) - MODULE_ATTRS
