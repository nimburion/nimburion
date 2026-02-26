#!/usr/bin/env python3
"""
Generate godoc comments for public Go symbols using AI.
"""
import os
import re
import sys
from pathlib import Path

def extract_public_symbols(file_path):
    """Extract public functions/types/methods without godoc comments."""
    with open(file_path, 'r') as f:
        lines = f.readlines()
    
    symbols = []
    for i, line in enumerate(lines):
        # Skip if previous line is a comment
        if i > 0 and lines[i-1].strip().startswith('//'):
            continue
            
        # Match public functions
        if re.match(r'^func\s+[A-Z]', line):
            symbols.append({
                'line': i + 1,
                'type': 'function',
                'code': line.strip(),
                'file': file_path
            })
        # Match public methods
        elif re.match(r'^func\s+\([^)]+\)\s+[A-Z]', line):
            symbols.append({
                'line': i + 1,
                'type': 'method',
                'code': line.strip(),
                'file': file_path
            })
        # Match public types
        elif re.match(r'^type\s+[A-Z]', line):
            symbols.append({
                'line': i + 1,
                'type': 'type',
                'code': line.strip(),
                'file': file_path
            })
    
    return symbols

def find_go_files(root_dir):
    """Find all non-test Go files."""
    go_files = []
    for path in Path(root_dir).rglob('*.go'):
        if not path.name.endswith('_test.go'):
            go_files.append(str(path))
    return go_files

def main():
    repo_root = Path(__file__).parent.parent
    pkg_dir = repo_root / 'pkg'
    
    # Focus on key packages first
    priority_packages = [
        'server',
        'config',
        'auth',
        'middleware',
        'jobs',
        'scheduler',
        'eventbus',
    ]
    
    all_symbols = []
    for pkg in priority_packages:
        pkg_path = pkg_dir / pkg
        if not pkg_path.exists():
            continue
            
        for go_file in pkg_path.rglob('*.go'):
            if go_file.name.endswith('_test.go'):
                continue
            symbols = extract_public_symbols(str(go_file))
            all_symbols.extend(symbols)
    
    # Print summary
    print(f"Found {len(all_symbols)} public symbols without godoc comments")
    print("\nBreakdown by type:")
    types = {}
    for sym in all_symbols:
        types[sym['type']] = types.get(sym['type'], 0) + 1
    for t, count in types.items():
        print(f"  {t}: {count}")
    
    # Print first 20 for review
    print("\nFirst 20 symbols:")
    for sym in all_symbols[:20]:
        print(f"{sym['file']}:{sym['line']} - {sym['code'][:80]}")

if __name__ == '__main__':
    main()
