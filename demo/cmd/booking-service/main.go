// Command booking-service runs the demo end to end: it publishes a few trade
// events, consumes them, replays the topic once (as a rebalance would), and
// prints the resulting book and balances.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/shopspring/decimal"

	"github.com/bigzoo/gca-26/demo/internal/booking"
	"github.com/bigzoo/gca-26/demo/internal/events"
	"github.com/bigzoo/gca-26/demo/internal/ledger"
)

func main() {
	ctx := context.Background()
	book := ledger.New()
	bus := events.New()
	svc := booking.New(book)

	// Opening balance: the client wired 10,000 KES to us.
	must1(book.Post(ledger.Entry{Debit: "cash:acc-1:KES", Credit: "clientmoney:bank:KES", Amount: d("10000"), Currency: "KES", Memo: "deposit"}))

	publish(bus, "evt-001", booking.TradeExecuted{TradeID: "T-1", Account: "acc-1", Instrument: "SAF", Side: "BUY", Quantity: d("100"), Price: d("18.50"), Fee: d("9.25"), Currency: "KES"})
	publish(bus, "evt-002", booking.TradeExecuted{TradeID: "T-2", Account: "acc-1", Instrument: "KCB", Side: "BUY", Quantity: d("40"), Price: d("42.00"), Fee: d("8.40"), Currency: "KES"})
	publish(bus, "evt-003", booking.TradeExecuted{TradeID: "T-1", Account: "acc-1", Instrument: "SAF", Side: "SELL", Quantity: d("30"), Price: d("19.10"), Fee: d("2.87"), Currency: "KES"})

	must(bus.Consume(ctx, booking.Topic, 0, svc.Handle))
	fmt.Println("consumed", bus.Len(booking.Topic), "events; replaying topic (simulated rebalance)...")
	must(bus.Replay(ctx, booking.Topic, svc.Handle))

	fmt.Println()
	fmt.Println("BOOK")
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "PRIMANOTA\tDEBIT\tCREDIT\tAMOUNT\tMEMO")
	for _, e := range book.Entries() {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s %s\t%s\n", e.Primanota, e.Debit, e.Credit, e.Amount.StringFixed(2), e.Currency, e.Memo)
	}
	w.Flush()

	fmt.Println()
	fmt.Println("BALANCES (KES)")
	for _, a := range []ledger.AccountID{"cash:acc-1:KES", "sec:acc-1:SAF", "sec:acc-1:KCB", "fees:house:KES", "clientmoney:bank:KES"} {
		fmt.Printf("  %-22s %12s\n", a, book.Balance(a, "KES").StringFixed(2))
	}
}

func publish(bus *events.Bus, id string, t booking.TradeExecuted) {
	p, _ := json.Marshal(t)
	bus.Publish(booking.Topic, events.Event{ID: id, Type: booking.TypeTradeExecuted, Key: t.Account, Payload: p})
}

func d(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func must1(_ ledger.PrimanotaID, err error) { must(err) }

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
