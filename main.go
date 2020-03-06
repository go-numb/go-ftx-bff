package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/go-numb/go-ftx-bff/reciever"
	"github.com/sirupsen/logrus"

	"time"

	_ "github.com/mattn/go-sqlite3"

	"golang.org/x/sync/errgroup"
)

var (
	TABLE string = "ftx_move"
)

type Client struct {
	db *sql.DB
}

func main() {
	database, _ := sql.Open("sqlite3", "./data/move.db")
	client := &Client{
		db: database,
	}
	defer client.db.Close()

	log := logrus.New()
	if err := client.Init(); err != nil {
		log.Error(err)
	}

	// 購読channelを更新して再購読
RECONNECT:

	ch := make(chan reciever.WsWriter)

	ctx, cancel := context.WithCancel(context.Background())

	go reciever.Connect(
		ctx,
		ch,
		[]string{"orderbook", "ticker", "trades"},
		[]string{ // UTC基準で今日と明日の出来高を取得して、ボラの拡大・縮小を見込む
			fmt.Sprintf("BTC-MOVE-%s", time.Now().UTC().Format("0102")),
			fmt.Sprintf("BTC-MOVE-%s", time.Now().UTC().Add(24*time.Hour).Format("0102")),
		}, log)

	var eg errgroup.Group
	eg.Go(func() error {
		y, m, d := time.Now().UTC().Add(24 * time.Hour).Date()
		tomorrow := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)

		for {
			select {
			case <-time.After(tomorrow.Sub(time.Now().UTC())):
				return fmt.Errorf("ended today, restart subscription")

			case v := <-ch:
				switch v.Types {

				case reciever.Unsubscribe:
					fmt.Printf("unsubscribe	-	%+v\n", v.Results.Error())
				case reciever.Subscribe:
					fmt.Printf("subscribe	-	%+v\n", v.Results.Error())

				case reciever.Partial, reciever.Update, reciever.Complete:
					switch v.Table {
					case "orderbook":
						// fmt.Printf("%s:orderbook:%s	-	%+v	-	%v\n", v.Types, v.ProductCode, v.Board, v.Board.Time)
					case "ticker":
						// fmt.Printf("%s:ticker:%s	-	%+v	-	%v\n", v.Types, v.ProductCode, v.Ticker, v.Ticker.Time)
					case "trades":
						if len(v.Trades) == 0 {
							continue
						}
						fmt.Printf("%s:trades:%s	-	%+v	-	%v\n", v.Types, v.ProductCode, v.Trades, v.Trades[0].Time)

						for i := range v.Trades {
							if err := client.Set(v.ProductCode, v.Table, v.Trades[i]); err != nil {
								log.Error(err)
							}
						}

					default:
						// fmt.Printf("default %s	-	%+v\n", v.Types, v)
					}

				case reciever.Undefined:
					// fmt.Printf("undefined	-	%+v\n", v)

				default:
					fmt.Printf("default	-	%+v\n", v)
				}

			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	if err := eg.Wait(); err != nil {
		log.Error(err)
	}

	// 購読をUnsubscibeしてRECONNECT
	cancel()
	time.Sleep(10 * time.Second)
	close(ch)

	goto RECONNECT
}

// Init 取得した一日限MOVE約定をtime.Unix()をIDにしたbyteで保存する
func (p *Client) Init() error {
	_, err := p.db.Exec(fmt.Sprintf(`create table %s (id integer not null primary key, product text, channel text, data blob);`, TABLE))
	if err != nil {
		return err
	}
	return nil
}

func (p *Client) Set(product, channel string, in interface{}) error {
	tx, err := p.db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(fmt.Sprintf("insert into %s (id, product, channel, data) values (?, ?, ?, ?);", TABLE))
	if err != nil {
		return err
	}
	defer stmt.Close()

	body, err := json.Marshal(in)
	if err != nil {
		return err
	}
	_, err = stmt.Exec(time.Now().UTC().Unix(), product, channel, body)
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

type Data struct {
	ID          int64
	ProductCode string
	Channel     string
	Body        []byte
}

func (p *Client) ReadInTerm(start, end time.Time) ([]Data, error) {
	//  where id < %d AND id > %d;
	rows, err := p.db.Query(fmt.Sprintf(`select id, product, channel, data from %s;`, TABLE)) // , start.UTC().Unix(), end.UTC().Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var datas []Data
	for rows.Next() {
		var data Data

		if err := rows.Scan(&data.ID, &data.ProductCode, &data.Channel, &data.Body); err != nil {
			return nil, err
		}
		datas = append(datas, data)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return datas, nil
}

// func (p *Client) ReadRow(id int, channel string) ([]byte, error) {
// 	stmt, err := p.db.Prepare(fmt.Sprintf(`select data from %s where id = ? AND channel = ?`, TABLE))
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer stmt.Close()

// 	var data []byte
// 	if err := stmt.QueryRow(id, channel).Scan(&data); err != nil {
// 		return nil, err
// 	}

// 	return data, nil
// }
