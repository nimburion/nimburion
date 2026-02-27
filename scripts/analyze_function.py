#!/usr/bin/env python3
"""
Analyze Go functions and generate accurate godoc comments based on implementation.
"""
import re
import sys
from pathlib import Path
from typing import Optional, Dict, List

def extract_function_body(lines: List[str], start_idx: int) -> str:
    """Extract function body for analysis."""
    body = []
    brace_count = 0
    started = False
    
    for i in range(start_idx, len(lines)):
        line = lines[i]
        if '{' in line:
            started = True
            brace_count += line.count('{')
        if started:
            body.append(line)
            brace_count -= line.count('}')
            if brace_count == 0:
                break
    
    return ''.join(body)

def analyze_function(name: str, signature: str, body: str) -> str:
    """Analyze function implementation and generate accurate comment."""
    body_lower = body.lower()
    
    # Analyze return patterns
    returns_error = 'error' in signature and 'error' in body
    returns_bool = re.search(r'\)\s+bool', signature)
    
    # Analyze behavior patterns
    patterns = {
        'validates': any(x in body_lower for x in ['validate', 'check', 'verify', 'ensure']),
        'creates': any(x in body_lower for x in ['new(', 'make(', '&']),
        'converts': any(x in body_lower for x in ['convert', 'transform', 'parse', 'marshal', 'unmarshal']),
        'filters': any(x in body_lower for x in ['filter', 'exclude', 'include', 'match']),
        'iterates': any(x in body_lower for x in ['for ', 'range']),
        'locks': any(x in body_lower for x in ['lock(', 'unlock(', 'mutex']),
        'closes': any(x in body_lower for x in ['close(', 'shutdown']),
        'starts': any(x in body_lower for x in ['start', 'run', 'serve', 'listen']),
        'stops': any(x in body_lower for x in ['stop', 'cancel', 'shutdown']),
        'reads': any(x in body_lower for x in ['read', 'get', 'fetch', 'load', 'query']),
        'writes': any(x in body_lower for x in ['write', 'set', 'put', 'save', 'store', 'insert', 'update']),
        'deletes': any(x in body_lower for x in ['delete', 'remove', 'drop']),
        'sends': any(x in body_lower for x in ['send', 'publish', 'emit', 'dispatch']),
        'receives': any(x in body_lower for x in ['receive', 'subscribe', 'listen', 'consume']),
        'blocks': any(x in body_lower for x in ['<-', 'wait', 'select {']),
        'goroutine': 'go ' in body,
        'defer': 'defer ' in body,
        'panic': 'panic(' in body,
        'recover': 'recover()' in body,
    }
    
    # Generate comment based on patterns
    if name.startswith('New'):
        type_name = name[3:]
        if patterns['validates']:
            return f"creates and validates a new {type_name} instance"
        return f"creates a new {type_name} instance"
    
    if name in ['Close', 'Shutdown']:
        if patterns['blocks']:
            return "gracefully closes all resources and waits for cleanup to complete"
        return "closes and releases all resources held by this instance"
    
    if name == 'Start':
        if patterns['blocks']:
            return "starts the service and blocks until context is cancelled or an error occurs"
        if patterns['goroutine']:
            return "starts the service in background goroutines"
        return "starts the service"
    
    if name in ['HealthCheck', 'Ping']:
        return "verifies the service is operational and can handle requests"
    
    if name == 'Router':
        return "returns the underlying router for registering custom routes"
    
    # HTTP methods
    if name in ['GET', 'POST', 'PUT', 'DELETE', 'PATCH']:
        return f"registers a handler for HTTP {name} requests at the specified path"
    
    if name == 'Group':
        return "creates a new router group with the given prefix and middleware"
    
    if name == 'Use':
        return "adds middleware that will be applied to all subsequent routes"
    
    if name == 'ServeHTTP':
        return "implements http.Handler by routing requests to registered handlers"
    
    # Context methods
    if name == 'Request':
        return "returns the HTTP request being processed"
    
    if name == 'Response':
        return "returns the response writer for sending the HTTP response"
    
    if name == 'SetRequest':
        return "replaces the HTTP request in this context"
    
    if name == 'SetResponse':
        return "replaces the response writer in this context"
    
    if name == 'Param':
        return "retrieves a URL path parameter by name"
    
    if name == 'Query':
        return "retrieves a URL query parameter by name"
    
    if name == 'Bind':
        return "deserializes the request body into the provided value based on Content-Type"
    
    if name == 'JSON':
        return "serializes the value as JSON and writes it with the given status code"
    
    if name == 'String':
        return "writes a plain text response with the given status code"
    
    if name in ['Get', 'GetString', 'GetInt']:
        return "retrieves a value from the context by key"
    
    if name == 'Set':
        return "stores a value in the context with the given key"
    
    # ResponseWriter methods
    if name == 'Header':
        return "returns the response headers that can be modified before writing"
    
    if name == 'WriteHeader':
        return "sends the HTTP status code. Must be called before Write"
    
    if name == 'Write':
        return "writes data to the response body. Implements io.Writer"
    
    if name == 'Status':
        return "returns the HTTP status code that was written, or 0 if not yet written"
    
    if name == 'Written':
        return "returns true if the response has been written"
    
    if name == 'Flush':
        return "sends any buffered data to the client immediately"
    
    if name == 'Hijack':
        return "takes over the connection for protocols like WebSocket. Implements http.Hijacker"
    
    # Event bus
    if name == 'Publish':
        if 'batch' in body_lower:
            return "publishes multiple messages to the topic in a single batch operation"
        return "publishes a message to the specified topic"
    
    if name == 'Subscribe':
        if patterns['goroutine']:
            return "subscribes to the topic and processes messages in a background goroutine"
        return "subscribes to the topic and invokes the handler for each message"
    
    if name == 'Unsubscribe':
        return "removes the subscription and stops receiving messages from the topic"
    
    # Jobs
    if name == 'Enqueue':
        return "adds a job to the queue for asynchronous processing"
    
    if name == 'Reserve':
        if patterns['blocks']:
            return "claims a job from the queue, blocking until one is available or context is cancelled"
        return "claims a job from the queue for processing"
    
    if name == 'Ack':
        return "acknowledges successful completion of the job"
    
    if name == 'Nack':
        return "rejects the job and schedules it for retry based on the retry policy"
    
    if name == 'Renew':
        return "extends the lease duration to prevent the job from being reclaimed"
    
    if name == 'MoveToDLQ':
        return "moves the failed job to the dead letter queue for manual inspection"
    
    # Generic patterns
    if patterns['validates']:
        if returns_error:
            return f"validates the configuration and returns an error if invalid"
        return f"validates and returns true if valid"
    
    if patterns['converts']:
        return f"converts and returns the transformed value"
    
    if patterns['filters']:
        return f"filters and returns matching items"
    
    # Default
    return f"TODO: analyze implementation"

def main():
    if len(sys.argv) < 2:
        print("Usage: python3 analyze_function.py <go_file>")
        sys.exit(1)
    
    file_path = Path(sys.argv[1])
    with open(file_path, 'r') as f:
        lines = f.readlines()
    
    for i, line in enumerate(lines):
        # Match public functions/methods
        match = re.match(r'^func\s+(?:\([^)]+\)\s+)?([A-Z]\w+)', line)
        if not match:
            continue
        
        name = match.group(1)
        signature = line.strip()
        
        # Extract body
        body = extract_function_body(lines, i)
        
        # Generate comment
        comment = analyze_function(name, signature, body)
        
        print(f"\n// {name} {comment}")
        print(signature[:100])

if __name__ == '__main__':
    main()
