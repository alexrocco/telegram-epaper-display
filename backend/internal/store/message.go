package store

import "time"

// Message is a single Telegram channel post we care about for display.
type Message struct {
	// ID is the Telegram message_id within the channel.
	ID int `json:"id"`
	// Text is the post text (caption for media posts).
	Text string `json:"text"`
	// Author is the post author signature, if the channel enables it.
	Author string `json:"author,omitempty"`
	// Date is when the message was posted.
	Date time.Time `json:"date"`
}
