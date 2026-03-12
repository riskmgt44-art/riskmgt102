// services/email_service.go
package services

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"os"

	"github.com/resend/resend-go/v2"
)

var (
	resendClient *resend.Client
	fromEmail    = "RiskMGT <noreply@yourdomain.com>" // CHANGE THIS to your verified domain
)

// InitEmailService initializes the Resend client
func InitEmailService(apiKey string) {
	if apiKey == "" {
		apiKey = os.Getenv("RESEND_API_KEY")
		if apiKey == "" {
			log.Println("WARNING: RESEND_API_KEY not set, emails will not be sent")
			return
		}
	}
	resendClient = resend.NewClient(apiKey)
	log.Println("✅ Email service initialized with Resend")
}

// InvitationEmailData holds data for invitation email template
type InvitationEmailData struct {
	FirstName     string
	LastName      string
	Email         string
	Role          string
	RoleDisplay   string
	TempPassword  string
	InviteLink    string
	Organization  string
	InviterName   string
	LoginURL      string
	SupportEmail  string
}

// SendInvitationEmail sends an invitation email to a new user
func SendInvitationEmail(to, inviterName, orgName string, data InvitationEmailData) error {
	if resendClient == nil {
		log.Printf("Email client not initialized, skipping email to %s", to)
		return nil
	}

	// Set defaults
	if data.LoginURL == "" {
		// Use environment variable or default
		loginURL := os.Getenv("APP_URL")
		if loginURL == "" {
			loginURL = "http://localhost:8080"
		}
		data.LoginURL = loginURL + "/login"
	}
	if data.SupportEmail == "" {
		data.SupportEmail = "support@" + extractDomain(fromEmail)
	}
	if data.InviteLink == "" {
		data.InviteLink = data.LoginURL + "?email=" + to
	}

	// Map role to display name
	roleDisplay := map[string]string{
		"superadmin":   "Super Administrator",
		"admin":        "Administrator",
		"risk_manager": "Risk Manager",
		"user":         "Team Member",
	}[data.Role]
	if roleDisplay == "" {
		roleDisplay = data.Role
	}
	data.RoleDisplay = roleDisplay

	// Generate HTML email
	htmlContent, err := generateInvitationHTML(data)
	if err != nil {
		return fmt.Errorf("failed to generate email HTML: %v", err)
	}

	// Generate plain text version
	textContent := generateInvitationText(data)

	// Send email via Resend
	params := &resend.SendEmailRequest{
		From:    fromEmail,
		To:      []string{to},
		Subject: fmt.Sprintf("You're invited to join %s on RiskMGT", orgName),
		Html:    htmlContent,
		Text:    textContent,
		Tags: []resend.Tag{
			{Name: "category", Value: "invitation"},
			{Name: "role", Value: data.Role},
		},
	}

	sent, err := resendClient.Emails.Send(params)
	if err != nil {
		return fmt.Errorf("failed to send email via Resend: %v", err)
	}

	log.Printf("✅ Invitation email sent to %s for role %s (ID: %s)", to, data.Role, sent.Id)
	return nil
}

// Helper function to extract domain from email
func extractDomain(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) > 1 {
		return strings.TrimSuffix(parts[1], ">")
	}
	return "riskmgt.com"
}

// generateInvitationHTML creates the HTML email template
func generateInvitationHTML(data InvitationEmailData) (string, error) {
	const emailTemplate = `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif;
            line-height: 1.6;
            color: #111111;
            margin: 0;
            padding: 0;
            background-color: #f8f9fa;
        }
        .container {
            max-width: 600px;
            margin: 20px auto;
            background: white;
            border-radius: 12px;
            overflow: hidden;
            box-shadow: 0 20px 25px -5px rgba(0,0,0,0.1), 0 10px 10px -5px rgba(0,0,0,0.04);
        }
        .header {
            background: linear-gradient(135deg, #5DA1A1 0%, #4e9393 100%);
            padding: 32px 24px;
            text-align: center;
        }
        .header h1 {
            margin: 0;
            color: white;
            font-size: 28px;
            font-weight: 700;
            letter-spacing: -0.5px;
        }
        .content {
            padding: 32px 24px;
        }
        .welcome-text {
            font-size: 18px;
            font-weight: 600;
            margin-bottom: 16px;
            color: #111111;
        }
        .details-box {
            background: #f8fafc;
            border: 1px solid #e2e8f0;
            border-radius: 12px;
            padding: 20px;
            margin: 24px 0;
        }
        .detail-item {
            display: flex;
            margin-bottom: 12px;
            padding-bottom: 12px;
            border-bottom: 1px solid #e2e8f0;
        }
        .detail-item:last-child {
            border-bottom: none;
            margin-bottom: 0;
            padding-bottom: 0;
        }
        .detail-label {
            width: 120px;
            font-weight: 500;
            color: #5DA1A1;
        }
        .detail-value {
            flex: 1;
            color: #111111;
            font-weight: 500;
        }
        .password-box {
            background: #fff3cd;
            border: 1px solid #ffe69c;
            border-radius: 8px;
            padding: 16px;
            margin: 20px 0;
            font-family: monospace;
            font-size: 18px;
            text-align: center;
            color: #856404;
        }
        .button {
            display: inline-block;
            background: #5DA1A1;
            color: white;
            text-decoration: none;
            padding: 14px 32px;
            border-radius: 8px;
            font-weight: 600;
            margin: 16px 0;
            text-align: center;
        }
        .button:hover {
            background: #4e9393;
        }
        .footer {
            text-align: center;
            padding: 24px;
            color: #6b7280;
            font-size: 14px;
            border-top: 1px solid #e2e8f0;
            background: white;
        }
        .security-note {
            font-size: 12px;
            color: #6b7280;
            margin-top: 16px;
            padding-top: 16px;
            border-top: 1px solid #e2e8f0;
        }
        .role-badge {
            display: inline-block;
            background: rgba(93,161,161,0.1);
            color: #5DA1A1;
            padding: 6px 14px;
            border-radius: 20px;
            font-weight: 600;
            font-size: 14px;
            margin-top: 8px;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Welcome to RiskMGT</h1>
        </div>
        
        <div class="content">
            <div class="welcome-text">
                Hello {{.FirstName}} {{.LastName}},
            </div>
            
            <p style="color: #555555; margin-bottom: 24px;">
                <strong>{{.InviterName}}</strong> has invited you to join <strong>{{.Organization}}</strong> on RiskMGT - 
                your comprehensive risk management platform.
            </p>

            <div class="role-badge">
                Role: {{.RoleDisplay}}
            </div>

            <div class="details-box">
                <div class="detail-item">
                    <span class="detail-label">Email</span>
                    <span class="detail-value">{{.Email}}</span>
                </div>
                <div class="detail-item">
                    <span class="detail-label">Role</span>
                    <span class="detail-value">{{.RoleDisplay}}</span>
                </div>
                <div class="detail-item">
                    <span class="detail-label">Organization</span>
                    <span class="detail-value">{{.Organization}}</span>
                </div>
            </div>

            <div style="text-align: center;">
                <p style="margin-bottom: 12px; color: #555555;">
                    <strong>Your temporary password:</strong>
                </p>
                <div class="password-box">
                    {{.TempPassword}}
                </div>
                <a href="{{.LoginURL}}" class="button">
                    Login to RiskMGT
                </a>
                <p style="color: #777777; font-size: 14px; margin-top: 8px;">
                    You'll be prompted to change your password on first login.
                </p>
            </div>

            <div class="security-note">
                <strong>🔒 Security Note:</strong> For your protection, this password is temporary. 
                Please change it immediately after logging in. Never share your password with anyone.
            </div>
        </div>

        <div class="footer">
            <p>© 2025 RiskMGT. All rights reserved.</p>
            <p style="margin-top: 8px;">
                Need help? Contact us at <a href="mailto:{{.SupportEmail}}" style="color: #5DA1A1;">{{.SupportEmail}}</a>
            </p>
        </div>
    </div>
</body>
</html>`

	tmpl, err := template.New("invitation").Parse(emailTemplate)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// generateInvitationText creates a plain text version of the email
func generateInvitationText(data InvitationEmailData) string {
	return fmt.Sprintf(`Hello %s %s,

%s has invited you to join %s on RiskMGT - your comprehensive risk management platform.

Your account details:
- Email: %s
- Role: %s
- Organization: %s

Your temporary password: %s

Login URL: %s

You'll be prompted to change your password on first login.

For security reasons, please change your password immediately after logging in.

Need help? Contact us at %s

© 2025 RiskMGT`,
		data.FirstName, data.LastName,
		data.InviterName, data.Organization,
		data.Email, data.RoleDisplay, data.Organization,
		data.TempPassword, data.LoginURL, data.SupportEmail)
}

// SendPasswordResetEmail sends a password reset email
func SendPasswordResetEmail(to, name, resetLink string) error {
	if resendClient == nil {
		log.Printf("Email client not initialized, skipping password reset to %s", to)
		return nil
	}

	htmlContent := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #5DA1A1; color: white; padding: 24px; text-align: center; border-radius: 12px 12px 0 0; }
        .content { padding: 32px 24px; background: white; border: 1px solid #e2e8f0; border-radius: 0 0 12px 12px; }
        .button { background: #5DA1A1; color: white; padding: 12px 24px; text-decoration: none; border-radius: 6px; display: inline-block; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Password Reset Request</h1>
        </div>
        <div class="content">
            <p>Hello %s,</p>
            <p>We received a request to reset your password for your RiskMGT account.</p>
            <p>Click the button below to reset your password. This link will expire in 1 hour.</p>
            <p style="text-align: center;">
                <a href="%s" class="button">Reset Password</a>
            </p>
            <p>If you didn't request this, please ignore this email or contact support.</p>
        </div>
    </div>
</body>
</html>`, name, resetLink)

	params := &resend.SendEmailRequest{
		From:    fromEmail,
		To:      []string{to},
		Subject: "Reset your RiskMGT password",
		Html:    htmlContent,
		Tags: []resend.Tag{
			{Name: "category", Value: "password-reset"},
		},
	}

	_, err := resendClient.Emails.Send(params)
	if err != nil {
		return fmt.Errorf("failed to send password reset email: %v", err)
	}

	log.Printf("✅ Password reset email sent to %s", to)
	return nil
}