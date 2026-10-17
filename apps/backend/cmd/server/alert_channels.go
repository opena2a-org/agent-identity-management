package main

import (
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// startAlertChannels builds the alert channel service on repos.AlertChannel,
// registers its delivery as an observer of every alert repos.Alert inserts,
// and starts its workers. Every service that writes alerts through the shared
// alert repository reaches the organization's channels this way, whether or
// not it goes through AlertService. frontendURL is FRONTEND_URL. emailService
// is the server's email service, which sends the email channels' messages; it
// is nil when the server has no email provider configured.
func startAlertChannels(repos *Repositories, frontendURL string, emailService domain.EmailService) *application.AlertChannelService {
	channels := application.NewAlertChannelService(repos.AlertChannel, frontendURL, emailService)
	repos.Alert.OnCreate(channels.Enqueue)
	channels.Start()
	return channels
}
