package biz

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/billingaccount"
	"github.com/looplj/axonhub/internal/ent/billinghold"
	"github.com/looplj/axonhub/internal/ent/ledgertransaction"
	"github.com/looplj/axonhub/internal/ent/paymentevent"
	"github.com/looplj/axonhub/internal/ent/paymentorder"
	"github.com/looplj/axonhub/internal/ent/usagebillingrecord"
	"github.com/looplj/axonhub/internal/ent/usagedailyaggregate"
)

type BillingReportServiceParams struct {
	Ent *ent.Client
}

type BillingReportService struct {
	ent *ent.Client
}

func NewBillingReportService(params BillingReportServiceParams) *BillingReportService {
	return &BillingReportService{ent: params.Ent}
}

type BillingReportFilter struct {
	From     *time.Time
	To       *time.Time
	Currency string
	Limit    int
}

type BillingCommercialReport struct {
	From        *time.Time                `json:"from,omitempty"`
	To          *time.Time                `json:"to,omitempty"`
	Currency    string                    `json:"currency"`
	Summary     BillingReportSummary      `json:"summary"`
	Daily       []BillingDailyReportRow   `json:"daily"`
	TopModels   []BillingTopModelRow      `json:"topModels"`
	TopProjects []BillingTopProjectRow    `json:"topProjects"`
	TopAPIKeys  []BillingTopAPIKeyRow     `json:"topApiKeys"`
	TopChannels []BillingTopChannelRow    `json:"topChannels"`
	TopUsers    []BillingTopUserReportRow `json:"topUsers"`
}

type BillingReportSummary struct {
	RechargeAmountMicros    int64 `json:"rechargeAmountMicros"`
	ConsumptionAmountMicros int64 `json:"consumptionAmountMicros"`
	NetMovementMicros       int64 `json:"netMovementMicros"`
	RefundAmountMicros      int64 `json:"refundAmountMicros"`
	FailedPaymentCount      int   `json:"failedPaymentCount"`
	FailedPaymentEventCount int   `json:"failedPaymentEventCount"`
	PendingHoldAmountMicros int64 `json:"pendingHoldAmountMicros"`
	PendingHoldCount        int   `json:"pendingHoldCount"`
}

type BillingDailyReportRow struct {
	Date                    string `json:"date"`
	RechargeAmountMicros    int64  `json:"rechargeAmountMicros"`
	ConsumptionAmountMicros int64  `json:"consumptionAmountMicros"`
	NetMovementMicros       int64  `json:"netMovementMicros"`
	FailedPaymentCount      int    `json:"failedPaymentCount"`
	FailedPaymentEventCount int    `json:"failedPaymentEventCount"`
}

type BillingTopModelRow struct {
	ModelID            string `json:"modelId"`
	ChargeAmountMicros int64  `json:"chargeAmountMicros"`
	RequestCount       int    `json:"requestCount"`
}

type BillingTopProjectRow struct {
	ProjectID          int    `json:"projectId"`
	ProjectName        string `json:"projectName"`
	ChargeAmountMicros int64  `json:"chargeAmountMicros"`
	RequestCount       int    `json:"requestCount"`
}

type BillingTopAPIKeyRow struct {
	APIKeyID           int    `json:"apiKeyId"`
	APIKeyName         string `json:"apiKeyName"`
	ChargeAmountMicros int64  `json:"chargeAmountMicros"`
	RequestCount       int    `json:"requestCount"`
}

type BillingTopChannelRow struct {
	ChannelID          int    `json:"channelId"`
	ChannelName        string `json:"channelName"`
	ChargeAmountMicros int64  `json:"chargeAmountMicros"`
	RequestCount       int    `json:"requestCount"`
}

type BillingTopUserReportRow struct {
	UserID                  int    `json:"userId"`
	Email                   string `json:"email"`
	RechargeAmountMicros    int64  `json:"rechargeAmountMicros"`
	ConsumptionAmountMicros int64  `json:"consumptionAmountMicros"`
	NetAmountMicros         int64  `json:"netAmountMicros"`
}

type BillingCSVExportDataset string

const (
	BillingCSVExportDatasetLedgerTransactions  BillingCSVExportDataset = "ledger_transactions"
	BillingCSVExportDatasetUsageBillingRecords BillingCSVExportDataset = "usage_billing_records"
	BillingCSVExportDatasetPaymentOrders       BillingCSVExportDataset = "payment_orders"
	BillingCSVExportDatasetPaymentEvents       BillingCSVExportDataset = "payment_events"
)

type BillingCSVExportInput struct {
	Dataset  BillingCSVExportDataset
	From     *time.Time
	To       *time.Time
	Currency string
	Limit    int
}

type BillingCSVExportPayload struct {
	FileName    string `json:"fileName"`
	Content     string `json:"content"`
	ContentType string `json:"contentType"`
}

func (s *BillingReportService) GetCommercialReport(ctx context.Context, filter BillingReportFilter) (*BillingCommercialReport, error) {
	limit := normalizeReportLimit(filter.Limit)
	currency := normalizeReportCurrency(filter.Currency)

	accounts, err := s.ent.BillingAccount.Query().
		Where(billingaccount.OwnerTypeEQ(billingaccount.OwnerTypeUser)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query billing accounts: %w", err)
	}
	accountOwner := map[int]int{}
	for _, account := range accounts {
		accountOwner[account.ID] = account.OwnerID
	}

	users, err := s.ent.User.Query().All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	userEmail := map[int]string{}
	for _, user := range users {
		userEmail[user.ID] = user.Email
	}

	projects, err := s.ent.Project.Query().All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query projects: %w", err)
	}
	projectName := map[int]string{}
	for _, project := range projects {
		projectName[project.ID] = project.Name
	}

	apiKeys, err := s.ent.APIKey.Query().All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query api keys: %w", err)
	}
	apiKeyName := map[int]string{}
	for _, apiKey := range apiKeys {
		apiKeyName[apiKey.ID] = apiKey.Name
	}

	channels, err := s.ent.Channel.Query().All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query channels: %w", err)
	}
	channelName := map[int]string{}
	for _, channel := range channels {
		channelName[channel.ID] = channel.Name
	}

	ledgerRows, err := s.queryLedgerTransactions(ctx, BillingCSVExportInput{From: filter.From, To: filter.To, Currency: currency})
	if err != nil {
		return nil, err
	}
	aggregateRows, err := s.queryDailyUsageAggregates(ctx, filter, currency)
	if err != nil {
		return nil, err
	}
	orderRows, err := s.queryPaymentOrders(ctx, BillingCSVExportInput{From: filter.From, To: filter.To, Currency: currency})
	if err != nil {
		return nil, err
	}
	eventRows, err := s.queryPaymentEvents(ctx, BillingCSVExportInput{From: filter.From, To: filter.To})
	if err != nil {
		return nil, err
	}
	holdRows, err := s.queryBillingHolds(ctx, filter)
	if err != nil {
		return nil, err
	}

	report := &BillingCommercialReport{
		From:     filter.From,
		To:       filter.To,
		Currency: currency,
	}
	dailyByDate := map[string]*BillingDailyReportRow{}
	models := map[string]*BillingTopModelRow{}
	projectsByID := map[int]*BillingTopProjectRow{}
	apiKeysByID := map[int]*BillingTopAPIKeyRow{}
	channelsByID := map[int]*BillingTopChannelRow{}
	usersByID := map[int]*BillingTopUserReportRow{}

	for _, tx := range ledgerRows {
		if tx.Status != ledgertransaction.StatusPosted {
			continue
		}
		date := reportDate(tx.CreatedAt)
		daily := dailyRow(dailyByDate, date)
		switch tx.Type {
		case ledgertransaction.TypePaymentRecharge:
			if tx.Direction == ledgertransaction.DirectionCredit {
				report.Summary.RechargeAmountMicros += tx.AmountMicros
				report.Summary.NetMovementMicros += tx.AmountMicros
				daily.RechargeAmountMicros += tx.AmountMicros
				daily.NetMovementMicros += tx.AmountMicros
			}
		case ledgertransaction.TypeUsageCharge:
			if tx.Direction == ledgertransaction.DirectionDebit {
				report.Summary.ConsumptionAmountMicros += tx.AmountMicros
				report.Summary.NetMovementMicros -= tx.AmountMicros
			}
		case ledgertransaction.TypeRefund, ledgertransaction.TypeChargeback:
			report.Summary.RefundAmountMicros += tx.AmountMicros
			if tx.Direction == ledgertransaction.DirectionDebit {
				report.Summary.NetMovementMicros -= tx.AmountMicros
				daily.NetMovementMicros -= tx.AmountMicros
			} else {
				report.Summary.NetMovementMicros += tx.AmountMicros
				daily.NetMovementMicros += tx.AmountMicros
			}
		default:
			if tx.Direction == ledgertransaction.DirectionCredit {
				report.Summary.NetMovementMicros += tx.AmountMicros
				daily.NetMovementMicros += tx.AmountMicros
			} else {
				report.Summary.NetMovementMicros -= tx.AmountMicros
				daily.NetMovementMicros -= tx.AmountMicros
			}
		}
	}

	for _, order := range orderRows {
		date := reportDate(order.CreatedAt)
		if order.Status == paymentorder.StatusFailed {
			report.Summary.FailedPaymentCount++
			dailyRow(dailyByDate, date).FailedPaymentCount++
		}
		if order.Status == paymentorder.StatusPaid {
			userID := accountOwner[order.BillingAccountID]
			if userID > 0 {
				row := userReportRow(usersByID, userID, userEmail[userID])
				row.RechargeAmountMicros += order.AmountMicros
				row.NetAmountMicros += order.AmountMicros
			}
		}
	}

	for _, event := range eventRows {
		if event.Status == paymentevent.StatusFailed {
			report.Summary.FailedPaymentEventCount++
			dailyRow(dailyByDate, reportDate(event.CreatedAt)).FailedPaymentEventCount++
		}
	}

	for _, hold := range holdRows {
		if hold.Status == billinghold.StatusHeld {
			report.Summary.PendingHoldCount++
			report.Summary.PendingHoldAmountMicros += hold.AmountMicros
		}
	}

	for _, aggregate := range aggregateRows {
		if aggregate.Status != usagedailyaggregate.StatusCharged {
			continue
		}
		date := reportDate(aggregate.BucketStart)
		daily := dailyRow(dailyByDate, date)
		daily.ConsumptionAmountMicros += aggregate.UserChargeMicros
		daily.NetMovementMicros -= aggregate.UserChargeMicros

		model := models[aggregate.ModelID]
		if model == nil {
			model = &BillingTopModelRow{ModelID: aggregate.ModelID}
			models[aggregate.ModelID] = model
		}
		model.ChargeAmountMicros += aggregate.UserChargeMicros
		model.RequestCount += int(aggregate.RequestCount)

		project := projectsByID[aggregate.ProjectID]
		if project == nil {
			project = &BillingTopProjectRow{ProjectID: aggregate.ProjectID, ProjectName: projectName[aggregate.ProjectID]}
			projectsByID[aggregate.ProjectID] = project
		}
		project.ChargeAmountMicros += aggregate.UserChargeMicros
		project.RequestCount += int(aggregate.RequestCount)

		if aggregate.APIKeyID > 0 {
			apiKey := apiKeysByID[aggregate.APIKeyID]
			if apiKey == nil {
				apiKey = &BillingTopAPIKeyRow{APIKeyID: aggregate.APIKeyID, APIKeyName: apiKeyName[aggregate.APIKeyID]}
				apiKeysByID[aggregate.APIKeyID] = apiKey
			}
			apiKey.ChargeAmountMicros += aggregate.UserChargeMicros
			apiKey.RequestCount += int(aggregate.RequestCount)
		}

		if aggregate.ChannelID > 0 {
			channel := channelsByID[aggregate.ChannelID]
			if channel == nil {
				channel = &BillingTopChannelRow{ChannelID: aggregate.ChannelID, ChannelName: channelName[aggregate.ChannelID]}
				channelsByID[aggregate.ChannelID] = channel
			}
			channel.ChargeAmountMicros += aggregate.UserChargeMicros
			channel.RequestCount += int(aggregate.RequestCount)
		}

		if aggregate.UserID > 0 {
			user := userReportRow(usersByID, aggregate.UserID, userEmail[aggregate.UserID])
			user.ConsumptionAmountMicros += aggregate.UserChargeMicros
			user.NetAmountMicros -= aggregate.UserChargeMicros
		}
	}

	report.Daily = mapDailyRows(dailyByDate)
	report.TopModels = topModelRows(models, limit)
	report.TopProjects = topProjectRows(projectsByID, limit)
	report.TopAPIKeys = topAPIKeyRows(apiKeysByID, limit)
	report.TopChannels = topChannelRows(channelsByID, limit)
	report.TopUsers = topUserRows(usersByID, limit)
	return report, nil
}

func (s *BillingReportService) ExportCSV(ctx context.Context, input BillingCSVExportInput) (*BillingCSVExportPayload, error) {
	input.Limit = normalizeCSVLimit(input.Limit)
	input.Currency = normalizeOptionalCurrency(input.Currency)

	var (
		name    string
		content string
		err     error
	)
	switch input.Dataset {
	case BillingCSVExportDatasetLedgerTransactions:
		name = "ledger-transactions"
		rows, queryErr := s.queryLedgerTransactions(ctx, input)
		if queryErr != nil {
			return nil, queryErr
		}
		content, err = ledgerTransactionsCSV(rows)
	case BillingCSVExportDatasetUsageBillingRecords:
		name = "usage-billing-records"
		rows, queryErr := s.queryUsageBillingRecords(ctx, input)
		if queryErr != nil {
			return nil, queryErr
		}
		content, err = usageBillingRecordsCSV(rows)
	case BillingCSVExportDatasetPaymentOrders:
		name = "payment-orders"
		rows, queryErr := s.queryPaymentOrders(ctx, input)
		if queryErr != nil {
			return nil, queryErr
		}
		content, err = paymentOrdersCSV(rows)
	case BillingCSVExportDatasetPaymentEvents:
		name = "payment-events"
		rows, queryErr := s.queryPaymentEvents(ctx, input)
		if queryErr != nil {
			return nil, queryErr
		}
		content, err = paymentEventsCSV(rows)
	default:
		return nil, fmt.Errorf("unsupported billing CSV dataset: %s", input.Dataset)
	}
	if err != nil {
		return nil, err
	}

	return &BillingCSVExportPayload{
		FileName:    fmt.Sprintf("axonhub-billing-%s-%s.csv", name, time.Now().UTC().Format("20060102T150405Z")),
		Content:     content,
		ContentType: "text/csv",
	}, nil
}

func (s *BillingReportService) queryLedgerTransactions(ctx context.Context, input BillingCSVExportInput) ([]*ent.LedgerTransaction, error) {
	query := s.ent.LedgerTransaction.Query()
	if input.From != nil {
		query.Where(ledgertransaction.CreatedAtGTE(*input.From))
	}
	if input.To != nil {
		query.Where(ledgertransaction.CreatedAtLTE(*input.To))
	}
	if input.Currency != "" {
		query.Where(ledgertransaction.CurrencyEQ(input.Currency))
	}
	if input.Limit > 0 {
		query.Limit(input.Limit)
	}
	rows, err := query.Order(ent.Desc(ledgertransaction.FieldCreatedAt)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query ledger transactions: %w", err)
	}
	return rows, nil
}

func (s *BillingReportService) queryUsageBillingRecords(ctx context.Context, input BillingCSVExportInput) ([]*ent.UsageBillingRecord, error) {
	query := s.ent.UsageBillingRecord.Query()
	if input.From != nil {
		query.Where(usagebillingrecord.CreatedAtGTE(*input.From))
	}
	if input.To != nil {
		query.Where(usagebillingrecord.CreatedAtLTE(*input.To))
	}
	if input.Currency != "" {
		query.Where(usagebillingrecord.CurrencyEQ(input.Currency))
	}
	if input.Limit > 0 {
		query.Limit(input.Limit)
	}
	rows, err := query.Order(ent.Desc(usagebillingrecord.FieldCreatedAt)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query usage billing records: %w", err)
	}
	return rows, nil
}

func (s *BillingReportService) queryDailyUsageAggregates(ctx context.Context, filter BillingReportFilter, currency string) ([]*ent.UsageDailyAggregate, error) {
	query := s.ent.UsageDailyAggregate.Query().
		Where(usagedailyaggregate.CurrencyEQ(currency))
	if filter.From != nil {
		query.Where(usagedailyaggregate.BucketStartGTE(truncateUsageDay(*filter.From)))
	}
	if filter.To != nil {
		query.Where(usagedailyaggregate.BucketStartLTE(truncateUsageDay(*filter.To)))
	}
	rows, err := query.Order(ent.Asc(usagedailyaggregate.FieldBucketStart)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query daily usage aggregates: %w", err)
	}
	return rows, nil
}

func (s *BillingReportService) queryPaymentOrders(ctx context.Context, input BillingCSVExportInput) ([]*ent.PaymentOrder, error) {
	query := s.ent.PaymentOrder.Query()
	if input.From != nil {
		query.Where(paymentorder.CreatedAtGTE(*input.From))
	}
	if input.To != nil {
		query.Where(paymentorder.CreatedAtLTE(*input.To))
	}
	if input.Currency != "" {
		query.Where(paymentorder.CurrencyEQ(input.Currency))
	}
	if input.Limit > 0 {
		query.Limit(input.Limit)
	}
	rows, err := query.Order(ent.Desc(paymentorder.FieldCreatedAt)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query payment orders: %w", err)
	}
	return rows, nil
}

func (s *BillingReportService) queryPaymentEvents(ctx context.Context, input BillingCSVExportInput) ([]*ent.PaymentEvent, error) {
	query := s.ent.PaymentEvent.Query()
	if input.From != nil {
		query.Where(paymentevent.CreatedAtGTE(*input.From))
	}
	if input.To != nil {
		query.Where(paymentevent.CreatedAtLTE(*input.To))
	}
	if input.Limit > 0 {
		query.Limit(input.Limit)
	}
	rows, err := query.Order(ent.Desc(paymentevent.FieldCreatedAt)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query payment events: %w", err)
	}
	return rows, nil
}

func (s *BillingReportService) queryBillingHolds(ctx context.Context, filter BillingReportFilter) ([]*ent.BillingHold, error) {
	query := s.ent.BillingHold.Query()
	if filter.From != nil {
		query.Where(billinghold.CreatedAtGTE(*filter.From))
	}
	if filter.To != nil {
		query.Where(billinghold.CreatedAtLTE(*filter.To))
	}
	if filter.Currency != "" {
		query.Where(billinghold.CurrencyEQ(filter.Currency))
	}
	rows, err := query.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query billing holds: %w", err)
	}
	return rows, nil
}

func ledgerTransactionsCSV(rows []*ent.LedgerTransaction) (string, error) {
	return writeCSV([]string{"id", "billing_account_id", "direction", "type", "status", "amount_micros", "currency", "reference_type", "reference_id", "memo", "created_by_type", "created_by_id", "created_at"}, func(w *csv.Writer) error {
		for _, row := range rows {
			if err := w.Write([]string{
				strconv.Itoa(row.ID),
				strconv.Itoa(row.BillingAccountID),
				string(row.Direction),
				string(row.Type),
				string(row.Status),
				strconv.FormatInt(row.AmountMicros, 10),
				row.Currency,
				row.ReferenceType,
				row.ReferenceID,
				row.Memo,
				string(row.CreatedByType),
				row.CreatedByID,
				formatCSVTime(row.CreatedAt),
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func usageBillingRecordsCSV(rows []*ent.UsageBillingRecord) (string, error) {
	return writeCSV([]string{"id", "usage_log_id", "billing_account_id", "project_id", "user_id", "api_key_id", "model_id", "request_type", "status", "charge_amount_micros", "cost_amount_micros", "currency", "ledger_transaction_id", "error", "created_at"}, func(w *csv.Writer) error {
		for _, row := range rows {
			if err := w.Write([]string{
				strconv.Itoa(row.ID),
				strconv.Itoa(row.UsageLogID),
				strconv.Itoa(row.BillingAccountID),
				strconv.Itoa(row.ProjectID),
				strconv.Itoa(row.UserID),
				strconv.Itoa(row.APIKeyID),
				row.ModelID,
				string(row.RequestType),
				string(row.Status),
				strconv.FormatInt(row.ChargeAmountMicros, 10),
				strconv.FormatInt(row.CostAmountMicros, 10),
				row.Currency,
				strconv.Itoa(row.LedgerTransactionID),
				row.Error,
				formatCSVTime(row.CreatedAt),
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func paymentOrdersCSV(rows []*ent.PaymentOrder) (string, error) {
	return writeCSV([]string{"order_no", "project_id", "billing_account_id", "provider_type", "status", "amount_micros", "currency", "external_trade_no", "created_at", "paid_at"}, func(w *csv.Writer) error {
		for _, row := range rows {
			if err := w.Write([]string{
				row.OrderNo,
				strconv.Itoa(row.ProjectID),
				strconv.Itoa(row.BillingAccountID),
				string(row.ProviderType),
				string(row.Status),
				strconv.FormatInt(row.AmountMicros, 10),
				row.Currency,
				stringPtrValue(row.ExternalTradeNo),
				formatCSVTime(row.CreatedAt),
				timePtrValue(row.PaidAt),
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func paymentEventsCSV(rows []*ent.PaymentEvent) (string, error) {
	return writeCSV([]string{"id", "event_key", "payment_order_id", "provider_instance_id", "provider_type", "event_type", "status", "error", "created_at"}, func(w *csv.Writer) error {
		for _, row := range rows {
			if err := w.Write([]string{
				strconv.Itoa(row.ID),
				row.EventKey,
				intPtrValue(row.PaymentOrderID),
				intPtrValue(row.ProviderInstanceID),
				string(row.ProviderType),
				row.EventType,
				string(row.Status),
				row.Error,
				formatCSVTime(row.CreatedAt),
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func writeCSV(header []string, writeRows func(*csv.Writer) error) (string, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	if err := writer.Write(header); err != nil {
		return "", err
	}
	if err := writeRows(writer); err != nil {
		return "", err
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func mapDailyRows(rows map[string]*BillingDailyReportRow) []BillingDailyReportRow {
	result := make([]BillingDailyReportRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, *row)
	}
	slices.SortFunc(result, func(a, b BillingDailyReportRow) int {
		if a.Date < b.Date {
			return -1
		}
		if a.Date > b.Date {
			return 1
		}
		return 0
	})
	return result
}

func topModelRows(rows map[string]*BillingTopModelRow, limit int) []BillingTopModelRow {
	result := make([]BillingTopModelRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, *row)
	}
	slices.SortFunc(result, func(a, b BillingTopModelRow) int {
		return compareReportRows(a.ChargeAmountMicros, b.ChargeAmountMicros, a.ModelID, b.ModelID)
	})
	return trimReportRows(result, limit)
}

func topProjectRows(rows map[int]*BillingTopProjectRow, limit int) []BillingTopProjectRow {
	result := make([]BillingTopProjectRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, *row)
	}
	slices.SortFunc(result, func(a, b BillingTopProjectRow) int {
		return compareReportRows(a.ChargeAmountMicros, b.ChargeAmountMicros, strconv.Itoa(a.ProjectID), strconv.Itoa(b.ProjectID))
	})
	return trimReportRows(result, limit)
}

func topAPIKeyRows(rows map[int]*BillingTopAPIKeyRow, limit int) []BillingTopAPIKeyRow {
	result := make([]BillingTopAPIKeyRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, *row)
	}
	slices.SortFunc(result, func(a, b BillingTopAPIKeyRow) int {
		return compareReportRows(a.ChargeAmountMicros, b.ChargeAmountMicros, strconv.Itoa(a.APIKeyID), strconv.Itoa(b.APIKeyID))
	})
	return trimReportRows(result, limit)
}

func topChannelRows(rows map[int]*BillingTopChannelRow, limit int) []BillingTopChannelRow {
	result := make([]BillingTopChannelRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, *row)
	}
	slices.SortFunc(result, func(a, b BillingTopChannelRow) int {
		return compareReportRows(a.ChargeAmountMicros, b.ChargeAmountMicros, strconv.Itoa(a.ChannelID), strconv.Itoa(b.ChannelID))
	})
	return trimReportRows(result, limit)
}

func topUserRows(rows map[int]*BillingTopUserReportRow, limit int) []BillingTopUserReportRow {
	result := make([]BillingTopUserReportRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, *row)
	}
	slices.SortFunc(result, func(a, b BillingTopUserReportRow) int {
		aTotal := a.RechargeAmountMicros + a.ConsumptionAmountMicros
		bTotal := b.RechargeAmountMicros + b.ConsumptionAmountMicros
		return compareReportRows(aTotal, bTotal, strconv.Itoa(a.UserID), strconv.Itoa(b.UserID))
	})
	return trimReportRows(result, limit)
}

func trimReportRows[T any](rows []T, limit int) []T {
	if limit <= 0 || len(rows) <= limit {
		return rows
	}
	return rows[:limit]
}

func compareReportRows(aAmount int64, bAmount int64, aKey string, bKey string) int {
	if aAmount > bAmount {
		return -1
	}
	if aAmount < bAmount {
		return 1
	}
	if aKey < bKey {
		return -1
	}
	if aKey > bKey {
		return 1
	}
	return 0
}

func dailyRow(rows map[string]*BillingDailyReportRow, date string) *BillingDailyReportRow {
	row := rows[date]
	if row == nil {
		row = &BillingDailyReportRow{Date: date}
		rows[date] = row
	}
	return row
}

func userReportRow(rows map[int]*BillingTopUserReportRow, userID int, email string) *BillingTopUserReportRow {
	row := rows[userID]
	if row == nil {
		row = &BillingTopUserReportRow{UserID: userID, Email: email}
		rows[userID] = row
	}
	return row
}

func normalizeReportLimit(limit int) int {
	if limit <= 0 {
		return 10
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func normalizeCSVLimit(limit int) int {
	if limit <= 0 {
		return 5000
	}
	if limit > 50000 {
		return 50000
	}
	return limit
}

func normalizeReportCurrency(currency string) string {
	if currency == "" {
		return "CNY"
	}
	return currency
}

func normalizeOptionalCurrency(currency string) string {
	return currency
}

func reportDate(value time.Time) string {
	return value.UTC().Format("2006-01-02")
}

func formatCSVTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339)
}

func timePtrValue(value *time.Time) string {
	if value == nil {
		return ""
	}
	return formatCSVTime(*value)
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func intPtrValue(value *int) string {
	if value == nil {
		return ""
	}
	return strconv.Itoa(*value)
}
