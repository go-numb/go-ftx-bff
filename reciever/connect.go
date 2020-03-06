package reciever

import (
	"context"
	"fmt"
	"time"

	"github.com/go-numb/go-ftx-bff/models"

	"github.com/buger/jsonparser"
	jsoniter "github.com/json-iterator/go"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/gorilla/websocket"
)

const (
	POINT = "wss://ftx.com/ws/"

	READDEADLINE  = 300
	WRITEDEADLINE = 5
	PINGTIMER     = 15
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

func subscribe(conn *websocket.Conn, channels, symbols []string) error {
	for i := range channels {
		for j := range symbols {
			if err := conn.WriteMessage(
				websocket.TextMessage,
				[]byte(fmt.Sprintf(`{"op": "subscribe", "channel": "%s", "market": "%s"}`, channels[i], symbols[j])),
			); err != nil {
				return err
			}
		}
	}

	return nil
}

func unsubscribe(conn *websocket.Conn, channels, symbols []string) error {
	for i := range channels {
		for j := range symbols {
			if err := conn.WriteMessage(
				websocket.TextMessage,
				[]byte(fmt.Sprintf(`{"op": "unsubscribe", "channel": "%s", "market": "%s"}`, channels[i], symbols[j])),
			); err != nil {
				return err
			}
		}
	}

	return nil
}

func ping(ctx context.Context, conn *websocket.Conn) error {
	ticker := time.NewTicker(time.Duration(PINGTIMER) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := conn.WriteControl(
				websocket.TextMessage,
				[]byte(`{"op":"ping"}`),
				time.Now().Add(time.Duration(WRITEDEADLINE)*time.Second),
			); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func New() *websocket.Conn {
	conn, _, err := websocket.DefaultDialer.Dial(POINT, nil)
	if err != nil {
		return nil
	}
	return conn
}

// channel
// market
// type: The type of message
// error: Occurs when there is an error. When type is error, there will also be a code and msg field. code takes on the values of standard HTTP error codes.
// subscribed: Indicates a successful subscription to the channel and market.
// unsubscribed: Indicates a successful unsubscription to the channel and market.
// info: Used to convey information to the user. Is accompanied by a code and msg field.
// When our servers restart, you may see an info message with code 20001. If you do, please reconnect.
// partial: Contains a snapshot of current market data. The data snapshot can be found in the accompanying data field.
// update: Contains an update about current market data. The update can be found in the accompanying data field.
// code (optional)
// msg (optional)
// data(optional)

type Types int

const (
	Error Types = iota
	Undefined
	Subscribe
	Unsubscribe
	Info
	Partial
	Update
	Complete
)

func (p Types) String() string {
	switch p {
	case Error:
		return "error"
	case Subscribe:
		return "subscribed"
	case Unsubscribe:
		return "unsubscribed"
	case Info:
		return "info"
	case Partial:
		return "partial"
	case Update:
		return "update"
	case Complete:
		return "complete"
	}

	return "undefined"
}

type WsWriter struct {
	Types       Types
	ProductCode string
	Table       string

	Board  models.Board
	Ticker models.Ticker
	Trades []models.Trade

	Results error
}

func Connect(ctx context.Context, ch chan WsWriter, channels, symbols []string, log *logrus.Logger) {
	conn := New()
	if conn == nil {
		log.Fatal(fmt.Errorf("conn has nil"))
	}
	defer conn.Close()

	if err := subscribe(conn, channels, symbols); err != nil {
		log.Fatal(err)
	}

	var eg errgroup.Group
	eg.Go(func() error { // ping every 15 seconds...
		if err := ping(ctx, conn); err != nil {
			return err
		}
		return nil
	})

	eg.Go(func() error {
		for {
			conn.SetReadDeadline(time.Now().Add(time.Duration(READDEADLINE) * time.Second))
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return err
			}

			var w WsWriter
			t, _ := jsonparser.GetString(msg, "type")
			channel, _ := jsonparser.GetString(msg, "channel")
			w.ProductCode, _ = jsonparser.GetString(msg, "market")
			data, _, _, _ := jsonparser.Get(msg, "data")

			switch t {
			case "error":
				w.Types = Error
				w.Results = fmt.Errorf(string(msg))
			case "subscribed":
				w.Types = Subscribe
				w.Results = fmt.Errorf(string(msg))
			case "unsubscribed":
				w.Types = Unsubscribe
				w.Results = fmt.Errorf(string(msg))
			case "info":
				w.Types = Info
				w.Results = fmt.Errorf(string(msg))
			case "partial":
				w.Types = Partial

			case "update":
				w.Types = Update

			default:
				w.Types = Undefined
				w.Results = fmt.Errorf(string(msg))
			}

			// Unmarshal
			if w.Types == Partial || w.Types == Update || w.Types == Undefined {
				switch channel {
				case "orderbook":
					w.Table = "orderbook"
					if err := json.Unmarshal(data, &w.Board); err != nil {
						w.Types = Error
						w.Results = fmt.Errorf("%v: %s", err, string(msg))
					}

				case "ticker":
					w.Table = "ticker"
					if err := json.Unmarshal(data, &w.Ticker); err != nil {
						w.Types = Error
						w.Results = fmt.Errorf("%v: %s", err, string(msg))
					}

				case "trades":
					w.Table = "trades"
					if err := json.Unmarshal(data, &w.Trades); err != nil {
						w.Types = Error
						w.Results = fmt.Errorf("%v: %s", err, string(msg))
					}

				}
			}

			ch <- w
		}
	})

	if err := eg.Wait(); err != nil {
		log.Error(err)
	}
}
