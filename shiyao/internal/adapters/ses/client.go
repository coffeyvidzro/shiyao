package ses

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

type Client struct {
	client *sesv2.Client
	from   string
}

func NewClient(cfg aws.Config, from string) *Client {
	return &Client{
		client: sesv2.NewFromConfig(cfg),
		from:   from,
	}
}

type Message struct {
	To      string
	Subject string
	HTML    string
	Text    string
}

func (c *Client) Send(
	ctx context.Context,
	message Message,
) error {
	_, err := c.client.SendEmail(ctx, &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(c.from),

		Destination: &types.Destination{
			ToAddresses: []string{
				message.To,
			},
		},

		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{
					Data: aws.String(message.Subject),
				},

				Body: &types.Body{
					Html: &types.Content{
						Data: aws.String(message.HTML),
					},
					Text: &types.Content{
						Data: aws.String(message.Text),
					},
				},
			},
		},
	})

	if err != nil {
		return fmt.Errorf("ses: send email: %w", err)
	}

	return nil
}
