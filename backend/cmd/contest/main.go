// Command contest runs one step of a contest's lifecycle (ADR-0016). Step Functions
// starts it as an ECS task at the contest's times; it can also be run by hand:
//
//	contest start|freeze|end|finalize <contest-id>
//
// Configuration (environment):
//
//	DATABASE_URL or DB_*       PostgreSQL
//	VALKEY_URL                 leaderboard cache (optional)
//	EVENT_BUS_NAME             EventBridge bus for phase events (optional)
//	CONTEST_EMAIL_FROM         verified SES sender for result e-mails (optional)
//	COGNITO_USER_POOL_ID       to look up participants' e-mail addresses (with CONTEST_EMAIL_FROM)
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sestypes "github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	"github.com/retek44/CodeArena/backend/internal/contest"
	"github.com/retek44/CodeArena/backend/internal/events"
	"github.com/retek44/CodeArena/backend/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(context.Background(), logger, os.Args[1:]); err != nil {
		logger.Error("contest step failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *slog.Logger, args []string) error {
	if len(args) != 2 {
		return errors.New("usage: contest start|freeze|end|finalize <contest-id>")
	}
	id, err := strconv.Atoi(args[1])
	if err != nil {
		return fmt.Errorf("contest id: %w", err)
	}

	dbURL, err := store.DatabaseURLFromEnv()
	if err != nil {
		return err
	}
	st, err := store.Open(ctx, dbURL)
	if err != nil {
		return err
	}
	defer st.Close()

	var cache contest.Leaderboard
	cache.Queries, cache.Logger = st.Queries, logger
	if url := os.Getenv("VALKEY_URL"); url != "" {
		if client, err := contest.NewCache(url); err == nil {
			defer client.Close()
			cache.Cache = client
		} else {
			logger.Warn("valkey unavailable", "error", err)
		}
	}

	lc := &contest.Lifecycle{Store: st, Leaderboard: &cache, Logger: logger}

	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("load AWS config: %w", err)
	}
	if bus := os.Getenv("EVENT_BUS_NAME"); bus != "" {
		lc.Events = &events.EventBridge{Client: eventbridge.NewFromConfig(awsCfg), Bus: bus}
	}
	if from, pool := os.Getenv("CONTEST_EMAIL_FROM"), os.Getenv("COGNITO_USER_POOL_ID"); from != "" && pool != "" {
		lc.Mailer = &sesMailer{client: sesv2.NewFromConfig(awsCfg), from: from}
		lc.Directory = &cognitoDirectory{client: cognitoidentityprovider.NewFromConfig(awsCfg), pool: pool}
	}
	return lc.Run(ctx, args[0], id)
}

type sesMailer struct {
	client *sesv2.Client
	from   string
}

func (m *sesMailer) Send(ctx context.Context, to, subject, body string) error {
	_, err := m.client.SendEmail(ctx, &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(m.from),
		Destination:      &sestypes.Destination{ToAddresses: []string{to}},
		Content: &sestypes.EmailContent{Simple: &sestypes.Message{
			Subject: &sestypes.Content{Data: aws.String(subject)},
			Body:    &sestypes.Body{Text: &sestypes.Content{Data: aws.String(body)}},
		}},
	})
	return err
}

// cognitoDirectory finds the e-mail of a user by the Cognito sub (our user ID).
type cognitoDirectory struct {
	client *cognitoidentityprovider.Client
	pool   string
}

func (d *cognitoDirectory) Email(ctx context.Context, userID string) (string, error) {
	out, err := d.client.ListUsers(ctx, &cognitoidentityprovider.ListUsersInput{
		UserPoolId:      aws.String(d.pool),
		Filter:          aws.String(fmt.Sprintf("sub = %q", userID)),
		Limit:           aws.Int32(1),
		AttributesToGet: []string{"email"},
	})
	if err != nil || len(out.Users) == 0 {
		return "", err
	}
	for _, a := range out.Users[0].Attributes {
		if aws.ToString(a.Name) == "email" {
			return aws.ToString(a.Value), nil
		}
	}
	return "", nil
}
