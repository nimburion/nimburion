#!/usr/bin/env python3
"""
Batch generate godoc comments for Go files.
This script identifies public symbols without comments and generates appropriate godoc.
"""
import re
import sys
from pathlib import Path
from typing import List, Dict, Tuple

# Godoc comment templates based on symbol type
TEMPLATES = {
    'Router': 'Router returns the underlying router instance for registering custom routes and middleware.',
    'Start': 'Start begins serving HTTP requests on the configured address. Blocks until context is cancelled or an error occurs.',
    'Shutdown': 'Shutdown gracefully stops the server, waiting for active connections to complete within the context deadline.',
    'Close': 'Close releases all resources held by this instance. Should be called when the instance is no longer needed.',
    'HealthCheck': 'HealthCheck verifies the component is operational and can perform its intended function.',
    'Ping': 'Ping performs a basic connectivity check to verify the service is reachable.',
    'GET': 'GET registers a handler for HTTP GET requests at the specified path.',
    'POST': 'POST registers a handler for HTTP POST requests at the specified path.',
    'PUT': 'PUT registers a handler for HTTP PUT requests at the specified path.',
    'DELETE': 'DELETE registers a handler for HTTP DELETE requests at the specified path.',
    'PATCH': 'PATCH registers a handler for HTTP PATCH requests at the specified path.',
    'Group': 'Group creates a new router group with the given prefix and optional middleware.',
    'Use': 'Use adds middleware to the router that will be applied to all subsequent routes.',
    'ServeHTTP': 'ServeHTTP implements the http.Handler interface, dispatching requests to registered handlers.',
    'Request': 'Request returns the underlying HTTP request being processed.',
    'Response': 'Response returns the response writer for sending HTTP responses.',
    'SetRequest': 'SetRequest updates the HTTP request associated with this context.',
    'SetResponse': 'SetResponse updates the response writer associated with this context.',
    'Param': 'Param retrieves a URL path parameter by name.',
    'Query': 'Query retrieves a URL query parameter by name.',
    'Bind': 'Bind deserializes the request body into the provided struct based on Content-Type.',
    'JSON': 'JSON serializes the given value as JSON and writes it to the response with the specified status code.',
    'String': 'String writes a plain text response with the specified status code.',
    'Get': 'Get retrieves a value from the context by key.',
    'Set': 'Set stores a value in the context with the given key.',
    'Header': 'Header returns the HTTP response headers that can be modified before writing the response.',
    'WriteHeader': 'WriteHeader sends an HTTP response header with the provided status code.',
    'Write': 'Write writes data to the response body. Implements io.Writer interface.',
    'Status': 'Status returns the HTTP status code that was written, or 0 if not yet written.',
    'Written': 'Written returns true if the response headers and body have been written.',
    'Flush': 'Flush sends any buffered data to the client immediately.',
    'Hijack': 'Hijack takes over the underlying connection for custom protocols like WebSocket.',
}

def generate_comment(symbol_name: str, symbol_type: str, code: str) -> str:
    """Generate appropriate godoc comment for a symbol."""
    
    # Check if we have a template for this method name
    if symbol_name in TEMPLATES:
        return f"// {symbol_name} {TEMPLATES[symbol_name]}"
    
    # Generate based on symbol type
    if symbol_type == 'method':
        # Extract receiver and method name
        match = re.match(r'func\s+\((\w+)\s+\*?(\w+)\)\s+(\w+)', code)
        if match:
            receiver_var, receiver_type, method_name = match.groups()
            return f"// {method_name} TODO: add description"
    
    elif symbol_type == 'function':
        match = re.match(r'func\s+(\w+)', code)
        if match:
            func_name = match.group(1)
            if func_name.startswith('New'):
                type_name = func_name[3:]  # Remove 'New' prefix
                return f"// {func_name} creates a new {type_name} instance."
            return f"// {func_name} TODO: add description"
    
    elif symbol_type == 'type':
        match = re.match(r'type\s+(\w+)', code)
        if match:
            type_name = match.group(1)
            return f"// {type_name} TODO: add description"
    
    return f"// TODO: add godoc comment"

def process_file(file_path: Path) -> List[Tuple[int, str, str]]:
    """Process a Go file and return list of (line_num, original_line, comment) tuples."""
    with open(file_path, 'r') as f:
        lines = f.readlines()
    
    changes = []
    for i, line in enumerate(lines):
        # Skip if previous line is already a comment
        if i > 0 and lines[i-1].strip().startswith('//'):
            continue
        
        symbol_name = None
        symbol_type = None
        
        # Match public functions
        if re.match(r'^func\s+[A-Z]', line):
            match = re.match(r'^func\s+([A-Z]\w+)', line)
            if match:
                symbol_name = match.group(1)
                symbol_type = 'function'
        
        # Match public methods
        elif re.match(r'^func\s+\([^)]+\)\s+[A-Z]', line):
            match = re.match(r'^func\s+\([^)]+\)\s+([A-Z]\w+)', line)
            if match:
                symbol_name = match.group(1)
                symbol_type = 'method'
        
        # Match public types
        elif re.match(r'^type\s+[A-Z]', line):
            match = re.match(r'^type\s+([A-Z]\w+)', line)
            if match:
                symbol_name = match.group(1)
                symbol_type = 'type'
        
        if symbol_name and symbol_type:
            comment = generate_comment(symbol_name, symbol_type, line.strip())
            changes.append((i + 1, line, comment))
    
    return changes

def main():
    if len(sys.argv) < 2:
        print("Usage: python3 generate_godoc_batch.py <go_file>")
        sys.exit(1)
    
    file_path = Path(sys.argv[1])
    if not file_path.exists():
        print(f"File not found: {file_path}")
        sys.exit(1)
    
    changes = process_file(file_path)
    
    if not changes:
        print(f"No public symbols without godoc found in {file_path}")
        return
    
    print(f"Found {len(changes)} symbols in {file_path}:")
    for line_num, original, comment in changes:
        print(f"\nLine {line_num}:")
        print(f"  {comment}")
        print(f"  {original.strip()[:80]}")

if __name__ == '__main__':
    main()
