package gql

import (
	"context"
	"fmt"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

func (r *quoteSubscriptionPromoInputResolver) PlanID(ctx context.Context, obj *biz.QuoteSubscriptionPromoInput, data *objects.GUID) error {
	if data == nil || data.Type != ent.TypeSubscriptionPlan {
		return fmt.Errorf("planId must be a SubscriptionPlan ID")
	}
	obj.PlanID = data.ID
	return nil
}

func promoQuoteFromApplication(app *biz.PromoApplication) *PromoQuote {
	quote := &PromoQuote{}
	if app == nil {
		return quote
	}
	if app.Code != nil {
		quote.Code = &app.Code.Code
	}
	quote.OriginalAmountMicros = int(app.OriginalAmountMicros)
	quote.DiscountAmountMicros = int(app.DiscountAmountMicros)
	quote.PayableAmountMicros = int(app.PayableAmountMicros)
	quote.Currency = app.Currency
	return quote
}
