package reciever

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"
)

func TestTime(t *testing.T) {
	y, m, d := time.Now().UTC().Add(24 * time.Hour).Date()
	tomorrow := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	fmt.Printf("%+v\n", tomorrow.Sub(time.Now().UTC()))
}

func TestConnect(t *testing.T) {
	done := make(chan os.Signal)
	ch := make(chan WsWriter)

	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		close(ch)
		close(done)
	}()

	go Connect(
		ctx,
		ch,
		[]string{"orderbook", "ticker", "trades"},
		[]string{ // UTC基準で今日と明日の出来高を取得して、ボラの拡大・縮小を見込む
			fmt.Sprintf("BTC-MOVE-%s", time.Now().UTC().Format("0102")),
			fmt.Sprintf("BTC-MOVE-%s", time.Now().UTC().Add(24*time.Hour).Format("0102")),
		}, nil)

	var eg errgroup.Group
	eg.Go(func() error {
		for {
			select {
			case v := <-ch:
				switch v.Types {

				case Unsubscribe:
					fmt.Printf("unsubscribe	-	%+v\n", v.Results.Error())
				case Subscribe:
					fmt.Printf("subscribe	-	%+v\n", v.Results.Error())

				case Partial, Update, Complete:
					switch v.Table {
					case "orderbook":
						// fmt.Printf("%s:orderbook:%s	-	%+v	-	%v\n", v.Types, v.ProductCode, v.Board, v.Board.Time)
					case "ticker":
						// fmt.Printf("%s:ticker:%s	-	%+v	-	%v\n", v.Types, v.ProductCode, v.Ticker, v.Ticker.Time)
					case "trades":
						fmt.Printf("%s:trades:%s	-	%+v	-	%v\n", v.Types, v.ProductCode, v.Trades, v.Trades[0].Time)

					default:
						fmt.Printf("default %s	-	%+v\n", v.Types, v)
					}

				case Undefined:
					fmt.Printf("undefined	-	%+v\n", v)
				default:
					fmt.Printf("default	-	%+v\n", v)
				}
			}
		}
	})

	<-done

}
