#!/usr/bin/env python3
"""
Fix all AuthGuard wrapper syntax issues in Next.js pages.

This script ensures:
1. Opening <AuthGuard> has properly indented child
2. Closing </AuthGuard> is present
3. Closing brace } is present after the return statement
"""

import re
from pathlib import Path

def fix_authguard_wrapper(file_path):
    """Fix AuthGuard wrapper in a single file."""
    with open(file_path, 'r') as f:
        content = f.read()

    # Pattern to find: return (\n    <AuthGuard>\n    <div
    # Should be: return (\n    <AuthGuard>\n      <div

    # Fix 1: Add indentation to first child after <AuthGuard>
    # Match pattern where <AuthGuard> is followed by non-indented child
    pattern1 = r'(  return \(\n    <AuthGuard>\n)    (<\w+)'
    replacement1 = r'\1      \2'
    content = re.sub(pattern1, replacement1, content)

    # Fix 2: Ensure </AuthGuard> closing tag exists
    # Check if file ends with just </div>\n  ); instead of </div>\n    </AuthGuard>\n  );\n}
    if '</AuthGuard>' not in content:
        # Find the last occurrence of   );\n and add </AuthGuard> before it
        content = re.sub(r'(\n    </div>\n)(  \);)$', r'\1    </AuthGuard>\n\2\n}', content)
    elif not content.rstrip().endswith('}'):
        # AuthGuard exists but missing closing brace
        if content.rstrip().endswith(');'):
            content = content.rstrip() + '\n}\n'

    with open(file_path, 'w') as f:
        f.write(content)

    print(f"Fixed: {file_path}")

def main():
    # Find all page.tsx files in app/dashboard
    app_dir = Path('apps/web/app/dashboard')
    page_files = list(app_dir.rglob('page.tsx'))

    print(f"Found {len(page_files)} page files")

    for page_file in page_files:
        try:
            fix_authguard_wrapper(page_file)
        except Exception as e:
            print(f"Error fixing {page_file}: {e}")

if __name__ == '__main__':
    main()
