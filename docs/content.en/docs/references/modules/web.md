---
title: "Web Module"
weight: 50
---

# Web Module

The Web module provides web interface capabilities including static file serving, WebSocket support, and web-based logging. It enables browser-based interaction with the framework through a comprehensive web application interface.

## Features

- Web application interface
- WebSocket real-time communication
- Live log streaming to browser
- Static file serving
- Integration with API endpoints
- Real-time system monitoring

## Configuration

The Web module uses the global `web` configuration section:

```yaml
web:
  enabled: true
  network:
    bind: "0.0.0.0:8080"
    publish: "localhost:8080"
  static_path: "./static"
  template_path: "./templates"
  websocket:
    enabled: true
    path: "/ws"
```

## Configuration Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `enabled` | boolean | `false` | Enable/disable the web module |
| `network.bind` | string | `"0.0.0.0:8080"` | Address and port to bind web server |
| `network.publish` | string | `"localhost:8080"` | Published address for external access |
| `static_path` | string | `"./static"` | Directory for static files |
| `template_path` | string | `"./templates"` | Directory for HTML templates |
| `websocket.enabled` | boolean | `true` | Enable WebSocket support |
| `websocket.path` | string | `"/ws"` | WebSocket endpoint path |

## Features in Detail

### WebSocket Integration

The Web module provides real-time communication through WebSockets:

- **Live log streaming** - Real-time log messages broadcast to connected clients
- **System status updates** - Real-time system health and statistics
- **Bi-directional communication** - Support for client-server messaging

### Web Application Interface

- **Dashboard** - System overview and monitoring interface
- **Configuration management** - Web-based configuration editing
- **Log viewer** - Real-time and historical log viewing
- **API explorer** - Interactive API documentation and testing

### Static File Serving

- **Asset management** - CSS, JavaScript, and image files
- **Template rendering** - Dynamic HTML page generation
- **Content caching** - Efficient static content delivery
- **MIME type detection** - Automatic content type handling

## WebSocket API

### Connection
```javascript
const ws = new WebSocket('ws://localhost:8080/ws');

ws.onopen = function(event) {
    console.log('WebSocket connected');
};

ws.onmessage = function(event) {
    console.log('Received:', event.data);
};
```

### Live Logging
The Web module automatically broadcasts log messages to connected WebSocket clients:

```javascript
ws.onmessage = function(event) {
    const logMessage = event.data;
    // Display log message in web interface
    appendLogMessage(logMessage);
};
```

## Usage Examples

### Basic Web Interface Access

```bash
# Access web interface
curl http://localhost:8080/

# Access specific static files
curl http://localhost:8080/css/style.css
curl http://localhost:8080/js/app.js
```

### WebSocket Connection

```html
<!DOCTYPE html>
<html>
<head>
    <title>Framework Monitor</title>
</head>
<body>
    <div id="logs"></div>
    
    <script>
        const ws = new WebSocket('ws://localhost:8080/ws');
        const logsDiv = document.getElementById('logs');
        
        ws.onmessage = function(event) {
            const logEntry = document.createElement('div');
            logEntry.textContent = event.data;
            logsDiv.appendChild(logEntry);
        };
    </script>
</body>
</html>
```

## Integration

The Web module integrates with:

- **API module** - Serves API endpoints alongside web interface
- **Stats module** - Displays real-time statistics and metrics
- **Logging system** - Provides live log streaming via WebSocket
- **Authentication system** - Protects web interface access

## Security Considerations

### Access Control
- Web interface can be protected by authentication
- Role-based access to different interface sections
- Session management and timeout handling

### HTTPS Support
```yaml
web:
  enabled: true
  tls:
    enabled: true
    cert_file: "/path/to/cert.pem"
    key_file: "/path/to/key.pem"
```

### Content Security
- Static file access restrictions
- Template injection prevention
- Cross-site scripting (XSS) protection

## Performance Features

### Caching
- Static file caching with ETags
- Template compilation caching
- Browser cache optimization

### Compression
- Gzip compression for text content
- Asset minification support
- Bandwidth optimization

## Development

### Custom Web Pages

Add custom pages by placing templates in the template directory:

```html
<!-- templates/custom.html -->
<!DOCTYPE html>
<html>
<head>
    <title>{{.Title}}</title>
</head>
<body>
    <h1>{{.Heading}}</h1>
    <p>{{.Content}}</p>
</body>
</html>
```

### Static Assets

Place static files in the configured static directory:
```
static/
├── css/
│   └── style.css
├── js/
│   └── app.js
└── images/
    └── logo.png
```

## Monitoring and Debugging

### Web Access Logs
Monitor web access through the stats module:
- Request counts and response times
- Error rates and status codes
- WebSocket connection statistics

### Health Checks
The web interface includes health monitoring:
- Server status indicators
- Real-time system metrics
- Connection status monitoring

## Troubleshooting

### Common Issues

1. **Web server not starting**
   - Check port availability and permissions
   - Verify configuration syntax
   - Review startup logs for errors

2. **Static files not loading**
   - Verify static_path configuration
   - Check file permissions
   - Ensure files exist in specified directory

3. **WebSocket connection failures**
   - Check WebSocket endpoint configuration
   - Verify browser WebSocket support
   - Review network connectivity

### Debug Mode

Enable debug mode for detailed logging:
```yaml
web:
  enabled: true
  debug: true
  log_requests: true
```

This provides detailed request logging and error information for troubleshooting web interface issues.