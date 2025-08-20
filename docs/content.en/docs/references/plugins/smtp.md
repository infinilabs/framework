---
title: "SMTP Plugin"
weight: 40
---

# SMTP Plugin

The SMTP plugin provides email notification capabilities for the framework, allowing you to send templated emails through SMTP servers as part of data processing pipelines. It supports multiple SMTP servers, custom templates, and variable substitution.

## Features

- SMTP email sending with authentication
- Multiple SMTP server configurations
- HTML and text email templates
- Variable substitution in templates
- Email attachments support
- TLS/SSL encryption support
- Template management and reusability
- Pipeline integration for automated notifications

## Configuration

Configure the SMTP plugin as a processor in your pipeline:

```yaml
pipelines:
  - name: "email_notifications"
    processors:
      - smtp:
          message_field: "messages"
          dial_timeout_in_seconds: 30
          variable_start_tag: "{{"
          variable_end_tag: "}}"
          
          variables:
            company_name: "INFINI Labs"
            support_email: "support@infinilabs.com"
          
          servers:
            primary:
              server:
                host: "smtp.gmail.com"
                port: 587
                tls: true
              auth:
                username: "notifications@example.com"
                password: "app_password"
              min_tls_version: "TLS12"
              sender: "noreply@example.com"
              recipients:
                to: ["admin@example.com"]
                cc: ["alerts@example.com"]
                bcc: []
          
          templates:
            alert:
              content_type: "html"
              subject: "{{alert_type}} Alert - {{service_name}}"
              body: |
                <html>
                <body>
                  <h2>Alert Notification</h2>
                  <p><strong>Service:</strong> {{service_name}}</p>
                  <p><strong>Alert Type:</strong> {{alert_type}}</p>
                  <p><strong>Message:</strong> {{message}}</p>
                  <p><strong>Time:</strong> {{timestamp}}</p>
                </body>
                </html>
              attachments:
                - name: "logs.txt"
                  path: "/var/log/app.log"
                  content_type: "text/plain"
```

## Configuration Parameters

### Global Configuration

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `message_field` | string | `"messages"` | Field containing message data |
| `dial_timeout_in_seconds` | int | `30` | SMTP connection timeout |
| `variable_start_tag` | string | `"{{"` | Template variable start delimiter |
| `variable_end_tag` | string | `"}}"` | Template variable end delimiter |
| `variables` | map | `{}` | Global template variables |

### Server Configuration

| Parameter | Type | Description |
|-----------|------|-------------|
| `server.host` | string | SMTP server hostname |
| `server.port` | int | SMTP server port (usually 587 for TLS, 25 for plain) |
| `server.tls` | boolean | Enable TLS encryption |
| `auth.username` | string | SMTP authentication username |
| `auth.password` | string | SMTP authentication password |
| `min_tls_version` | string | Minimum TLS version (TLS10, TLS11, TLS12, TLS13) |
| `sender` | string | Email sender address |
| `recipients.to` | []string | Primary recipients |
| `recipients.cc` | []string | Carbon copy recipients |
| `recipients.bcc` | []string | Blind carbon copy recipients |

### Template Configuration

| Parameter | Type | Description |
|-----------|------|-------------|
| `content_type` | string | Email content type ("text" or "html") |
| `subject` | string | Email subject (supports variables) |
| `body` | string | Email body content (supports variables) |
| `body_file` | string | Path to file containing email body template |
| `attachments` | []Attachment | Email attachments |

### Attachment Configuration

| Parameter | Type | Description |
|-----------|------|-------------|
| `name` | string | Attachment filename |
| `path` | string | Path to attachment file |
| `content_type` | string | MIME content type |

## Template Variables

Templates support variable substitution using configurable delimiters:

### Global Variables
```yaml
variables:
  company_name: "INFINI Labs"
  support_email: "support@infinilabs.com"
  environment: "production"
```

### Message Variables
Variables from the processed message are automatically available:
```yaml
# Message data
{
  "service_name": "elasticsearch",
  "alert_type": "high_cpu",
  "timestamp": "2024-01-01T12:00:00Z",
  "message": "CPU usage exceeded 90%"
}

# Template usage
subject: "{{alert_type}} Alert - {{service_name}}"
# Resolves to: "high_cpu Alert - elasticsearch"
```

## Usage Examples

### Simple Alert Email
```yaml
smtp:
  servers:
    default:
      server:
        host: "localhost"
        port: 587
        tls: true
      auth:
        username: "alerts@company.com"
        password: "password"
      sender: "noreply@company.com"
      recipients:
        to: ["admin@company.com"]
  
  templates:
    simple_alert:
      content_type: "text"
      subject: "System Alert"
      body: "Alert: {{message}} at {{timestamp}}"
```

### HTML Report Email
```yaml
smtp:
  templates:
    report:
      content_type: "html"
      subject: "Daily Report - {{date}}"
      body_file: "/templates/daily_report.html"
      attachments:
        - name: "report.pdf"
          path: "/reports/daily_{{date}}.pdf"
          content_type: "application/pdf"
```

### Multi-Server Configuration
```yaml
smtp:
  servers:
    primary:
      server:
        host: "smtp-primary.company.com"
        port: 587
        tls: true
      # ... auth and recipients
    
    backup:
      server:
        host: "smtp-backup.company.com"
        port: 587
        tls: true
      # ... auth and recipients
```

## Template Files

Store templates in external files for better management:

```html
<!-- /templates/alert.html -->
<!DOCTYPE html>
<html>
<head>
    <title>{{alert_type}} Alert</title>
</head>
<body>
    <div style="font-family: Arial, sans-serif;">
        <h1 style="color: red;">{{alert_type}} Alert</h1>
        <table border="1" style="border-collapse: collapse;">
            <tr>
                <td><strong>Service</strong></td>
                <td>{{service_name}}</td>
            </tr>
            <tr>
                <td><strong>Time</strong></td>
                <td>{{timestamp}}</td>
            </tr>
            <tr>
                <td><strong>Message</strong></td>
                <td>{{message}}</td>
            </tr>
        </table>
    </div>
</body>
</html>
```

## Security Considerations

### Authentication
- Use app-specific passwords for Gmail and similar services
- Store credentials securely using the keystore module
- Rotate passwords regularly

### TLS Configuration
```yaml
smtp:
  servers:
    secure:
      server:
        tls: true
      min_tls_version: "TLS12"  # Enforce modern TLS
```

### Credential Management
```yaml
smtp:
  servers:
    secure:
      auth:
        username: "${keystore.smtp_username}"
        password: "${keystore.smtp_password}"
```

## Integration

The SMTP plugin integrates with:

- **Pipeline system** - Processes messages and sends emails
- **Template system** - Supports variable substitution
- **Keystore module** - Securely stores SMTP credentials
- **Stats system** - Tracks email sending statistics

## Monitoring

Monitor SMTP operations through:
- Success/failure rates for email sending
- SMTP connection statistics
- Template processing times
- Attachment processing status

## Error Handling

Common error scenarios:
- **SMTP connection failures** - Network issues or wrong server configuration
- **Authentication failures** - Invalid credentials
- **Template errors** - Missing variables or syntax errors
- **Attachment issues** - Missing files or permission problems

## Troubleshooting

### Connection Issues
```yaml
smtp:
  dial_timeout_in_seconds: 60  # Increase timeout
  servers:
    debug:
      server:
        host: "smtp.example.com"
        port: 587
        tls: true
      # Enable debug logging
```

### Authentication Problems
1. Verify SMTP credentials
2. Check if app-specific passwords are required
3. Ensure SMTP is enabled on the email service
4. Review authentication method requirements

### Template Issues
- Verify variable names match message data
- Check template syntax and delimiters
- Test templates with sample data
- Review template file permissions

### Best Practices

1. **Server Configuration**: Configure backup SMTP servers for reliability
2. **Template Management**: Use external template files for complex emails
3. **Variable Naming**: Use consistent variable naming conventions
4. **Error Handling**: Implement retry logic for transient failures
5. **Security**: Use TLS and secure credential storage
6. **Testing**: Test email templates with sample data before deployment