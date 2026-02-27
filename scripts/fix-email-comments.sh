#!/bin/bash
# Add comments for email provider types in providers_extra.go

file="pkg/email/providers_extra.go"

# MailerSend
sed -i.bak '/^type MailerSendConfig struct {$/i\
// MailerSendConfig configures the MailerSend email provider
' "$file"

sed -i.bak '/^type MailerSendProvider struct {$/i\
// MailerSendProvider implements email sending via MailerSend API
' "$file"

sed -i.bak '/^func NewMailerSendProvider/i\
// NewMailerSendProvider creates a new MailerSend email provider
' "$file"

sed -i.bak '/^func (p \*MailerSendProvider) Send/i\
// Send sends an email via MailerSend API
' "$file"

sed -i.bak '/^func (p \*MailerSendProvider) Close/i\
// Close closes the MailerSend provider (no-op)
' "$file"

# Postmark
sed -i.bak '/^type PostmarkConfig struct {$/i\
// PostmarkConfig configures the Postmark email provider
' "$file"

sed -i.bak '/^type PostmarkProvider struct {$/i\
// PostmarkProvider implements email sending via Postmark API
' "$file"

sed -i.bak '/^func NewPostmarkProvider/i\
// NewPostmarkProvider creates a new Postmark email provider
' "$file"

sed -i.bak '/^func (p \*PostmarkProvider) Send/i\
// Send sends an email via Postmark API
' "$file"

sed -i.bak '/^func (p \*PostmarkProvider) Close/i\
// Close closes the Postmark provider (no-op)
' "$file"

# Continue for other providers...
rm -f "$file.bak"

echo "Added comments to providers_extra.go"
