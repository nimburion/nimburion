#!/usr/bin/env python3
"""
Apply godoc comments to Go files automatically.
"""
import re
import sys
from pathlib import Path
from typing import List, Tuple

# Godoc comment templates
TEMPLATES = {
    'Router': 'returns the underlying router instance for registering custom routes and middleware.',
    'Start': 'begins serving HTTP requests on the configured address. Blocks until context is cancelled or an error occurs.',
    'Shutdown': 'gracefully stops the server, waiting for active connections to complete within the context deadline.',
    'Close': 'releases all resources held by this instance. Should be called when the instance is no longer needed.',
    'HealthCheck': 'verifies the component is operational and can perform its intended function.',
    'Ping': 'performs a basic connectivity check to verify the service is reachable.',
    'GET': 'registers a handler for HTTP GET requests at the specified path.',
    'POST': 'registers a handler for HTTP POST requests at the specified path.',
    'PUT': 'registers a handler for HTTP PUT requests at the specified path.',
    'DELETE': 'registers a handler for HTTP DELETE requests at the specified path.',
    'PATCH': 'registers a handler for HTTP PATCH requests at the specified path.',
    'Group': 'creates a new router group with the given prefix and optional middleware.',
    'Use': 'adds middleware to the router that will be applied to all subsequent routes.',
    'ServeHTTP': 'implements the http.Handler interface, dispatching requests to registered handlers.',
    'Request': 'returns the underlying HTTP request being processed.',
    'Response': 'returns the response writer for sending HTTP responses.',
    'SetRequest': 'updates the HTTP request associated with this context.',
    'SetResponse': 'updates the response writer associated with this context.',
    'Param': 'retrieves a URL path parameter by name.',
    'Query': 'retrieves a URL query parameter by name.',
    'Bind': 'deserializes the request body into the provided struct based on Content-Type.',
    'JSON': 'serializes the given value as JSON and writes it to the response with the specified status code.',
    'String': 'writes a plain text response with the specified status code.',
    'Get': 'retrieves a value from the context by key.',
    'Set': 'stores a value in the context with the given key.',
    'Header': 'returns the HTTP response headers that can be modified before writing the response.',
    'WriteHeader': 'sends an HTTP response header with the provided status code.',
    'Write': 'writes data to the response body. Implements io.Writer interface.',
    'Status': 'returns the HTTP status code that was written, or 0 if not yet written.',
    'Written': 'returns true if the response headers and body have been written.',
    'Flush': 'sends any buffered data to the client immediately.',
    'Hijack': 'takes over the underlying connection for custom protocols like WebSocket.',
    'Publish': 'sends a message to the specified topic.',
    'Subscribe': 'registers a handler to receive messages from the specified topic.',
    'Unsubscribe': 'removes the subscription for the specified topic.',
    'Enqueue': 'adds a job to the processing queue.',
    'Ack': 'acknowledges successful processing of a job.',
    'Nack': 'rejects a job and schedules it for retry.',
    'Reserve': 'claims a job from the queue for processing.',
    'Renew': 'extends the lease duration for a job being processed.',
    'MoveToDLQ': 'moves a failed job to the dead letter queue.',
    'Routes': 'returns all registered routes.',
}

def generate_comment(symbol_name: str, code: str) -> str:
    """Generate appropriate godoc comment for a symbol."""
    if symbol_name in TEMPLATES:
        return f"// {symbol_name} {TEMPLATES[symbol_name]}"
    
    # For New* constructors
    if symbol_name.startswith('New'):
        type_name = symbol_name[3:]
        return f"// {symbol_name} creates a new {type_name} instance."
    
    return f"// {symbol_name} TODO: add description"

def apply_comments(file_path: Path) -> int:
    """Apply godoc comments to file. Returns number of comments added."""
    with open(file_path, 'r') as f:
        lines = f.readlines()
    
    new_lines = []
    added_count = 0
    
    for i, line in enumerate(lines):
        # Check if previous line is already a comment
        if i > 0 and new_lines[-1].strip().startswith('//'):
            new_lines.append(line)
            continue
        
        symbol_name = None
        
        # Match public methods
        match = re.match(r'^func\s+\([^)]+\)\s+([A-Z]\w+)', line)
        if match:
            symbol_name = match.group(1)
        # Match public functions
        elif re.match(r'^func\s+([A-Z]\w+)', line):
            match = re.match(r'^func\s+([A-Z]\w+)', line)
            if match:
                symbol_name = match.group(1)
        
        if symbol_name:
            comment = generate_comment(symbol_name, line.strip())
            new_lines.append(comment + '\n')
            added_count += 1
        
        new_lines.append(line)
    
    if added_count > 0:
        with open(file_path, 'w') as f:
            f.writelines(new_lines)
    
    return added_count

def main():
    if len(sys.argv) < 2:
        print("Usage: python3 apply_godoc.py <go_file_or_directory>")
        sys.exit(1)
    
    path = Path(sys.argv[1])
    
    if path.is_file():
        files = [path]
    elif path.is_dir():
        files = list(path.rglob('*.go'))
        files = [f for f in files if not f.name.endswith('_test.go')]
    else:
        print(f"Path not found: {path}")
        sys.exit(1)
    
    total_added = 0
    for file_path in files:
        added = apply_comments(file_path)
        if added > 0:
            print(f"{file_path}: added {added} comments")
            total_added += added
    
    print(f"\nTotal: added {total_added} godoc comments across {len(files)} files")

if __name__ == '__main__':
    main()
