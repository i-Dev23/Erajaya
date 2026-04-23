package usecase

import (
	"context"
	"fmt"
	"os"
	"pps-services-tokopedia/internal/service"
	"strconv"
	"strings"
	"time"
)

// ScheduledJobsUsecase defines scheduled jobs registration behavior.
type ScheduledJobsUsecase interface {
	SetupScheduledJobs()
}

type scheduledJobsUsecaseImpl struct {
	schedulerService service.SchedulerService
	cleanupUsecase   CleanupUsecase
	postgresService  service.PostgresService
	logger           service.Logger
}

// NewScheduledJobsUsecase creates a new ScheduledJobsUsecase.
func NewScheduledJobsUsecase(
	schedulerService service.SchedulerService,
	cleanupUsecase CleanupUsecase,
	postgresService service.PostgresService,
	logger service.Logger,
) ScheduledJobsUsecase {
	return &scheduledJobsUsecaseImpl{
		schedulerService: schedulerService,
		cleanupUsecase:   cleanupUsecase,
		postgresService:  postgresService,
		logger:           logger,
	}
}

// SetupScheduledJobs configures all scheduled jobs.
func (u *scheduledJobsUsecaseImpl) SetupScheduledJobs() {
	// Guard: run jobs only when RUN_JOBS_LOG_RETENTION == "Y"
	if strings.ToUpper(strings.TrimSpace(os.Getenv("RUN_JOBS_LOG_RETENTION"))) != "Y" {
		u.logger.Info("Log retention jobs are disabled by env", "env", "RUN_JOBS_LOG_RETENTION")
		return
	}
	// Resolve cron expressions from env (with defaults)
	httpCron := os.Getenv("HTTP_LOG_RETENTION_CRON")
	if httpCron == "" {
		httpCron = "0 0 1 * * *"
	}
	callbackCron := os.Getenv("CALLBACK_LOG_RETENTION_CRON")
	if callbackCron == "" {
		callbackCron = "0 10 1 * * *"
	}
	inquiryCron := os.Getenv("INQUIRY_LOG_RETENTION_CRON")
	if inquiryCron == "" {
		inquiryCron = "0 20 1 * * *"
	}
	paymentCron := os.Getenv("PAYMENT_LOG_RETENTION_CRON")
	if paymentCron == "" {
		paymentCron = "0 30 1 * * *"
	}

	// File log cleanup cron
	fileLogCron := os.Getenv("FILE_LOG_RETENTION_CRON")
	if fileLogCron == "" {
		fileLogCron = "0 40 1 * * *" // 1:40 AM daily
	}

	// Schedule HTTP logs cleanup job (cron from env)
	_, err := u.schedulerService.AddJobWithContext(httpCron, func(ctx context.Context) {
		if !u.postgresService.IsAvailable() {
			u.logger.Warn("Skipping HTTP logs cleanup — Postgres not available")
			return
		}
		u.logger.Info("Starting scheduled HTTP logs cleanup job")

		// Get days to keep from environment variable, default to 31 days
		daysToKeep := os.Getenv("HTTP_LOG_RETENTION_DAYS")
		if daysToKeep == "" {
			daysToKeep = "31"
		}
		daysToKeepInt, err := strconv.Atoi(daysToKeep)
		if err != nil {
			u.logger.Error("Failed to convert daysToKeep to int", "error", err)
			return
		}

		deletedCount, err := u.cleanupUsecase.CleanupOldHTTPLogs(ctx, daysToKeepInt)
		if err != nil {
			u.logger.Error("Failed to cleanup old HTTP logs", "error", err)
			return
		}

		u.logger.Info("HTTP logs cleanup job completed", "deletedCount", deletedCount, "daysToKeep", daysToKeep)
	})

	if err != nil {
		u.logger.Error("Failed to schedule HTTP logs cleanup job", "error", err)
	} else {
		u.logger.Info("HTTP logs cleanup job scheduled successfully", "schedule", "1:00 AM daily")
	}

	// Schedule Callback logs cleanup job (cron from env)
	_, err2 := u.schedulerService.AddJobWithContext(callbackCron, func(ctx context.Context) {
		if !u.postgresService.IsAvailable() {
			u.logger.Warn("Skipping Callback logs cleanup — Postgres not available")
			return
		}
		u.logger.Info("Starting scheduled Callback logs cleanup job")

		// Get days to keep from environment variable, default to 31 days
		daysToKeep := os.Getenv("CALLBACK_LOG_RETENTION_DAYS")
		if daysToKeep == "" {
			daysToKeep = "31"
		}
		daysToKeepInt, err := strconv.Atoi(daysToKeep)
		if err != nil {
			u.logger.Error("Failed to convert daysToKeep to int", "error", err)
			return
		}

		deletedCount, err := u.cleanupUsecase.CleanupOldCallbackLogs(ctx, daysToKeepInt)
		if err != nil {
			u.logger.Error("Failed to cleanup old Callback logs", "error", err)
			return
		}

		u.logger.Info("Callback logs cleanup job completed", "deletedCount", deletedCount, "daysToKeep", daysToKeep)
	})

	if err2 != nil {
		u.logger.Error("Failed to schedule Callback logs cleanup job", "error", err2)
	} else {
		u.logger.Info("Callback logs cleanup job scheduled successfully", "schedule", "1:10 AM daily")
	}

	// Schedule Inquiry and Payment logs cleanup jobs using shared retention env
	retention := os.Getenv("INQUIRY_AND_PAYMENT_LOG_RETENTION_DAYS")
	if retention == "" {
		retention = "31"
	}
	retentionDays, convErr := strconv.Atoi(retention)
	if convErr != nil {
		u.logger.Error("Failed to convert INQUIRY_AND_PAYMENT_LOG_RETENTION_DAYS to int", "error", convErr)
		return
	}

	// Inquiry cleanup (cron from env)
	_, err3 := u.schedulerService.AddJobWithContext(inquiryCron, func(ctx context.Context) {
		if !u.postgresService.IsAvailable() {
			u.logger.Warn("Skipping Inquiry logs cleanup — Postgres not available")
			return
		}
		u.logger.Info("Starting scheduled Inquiry logs cleanup job")
		deletedCount, err := u.cleanupUsecase.CleanupOldInquiryLogs(ctx, retentionDays)
		if err != nil {
			u.logger.Error("Failed to cleanup old Inquiry logs", "error", err)
			return
		}
		u.logger.Info("Inquiry logs cleanup job completed", "deletedCount", deletedCount, "daysToKeep", retentionDays)
	})
	if err3 != nil {
		u.logger.Error("Failed to schedule Inquiry logs cleanup job", "error", err3)
	} else {
		u.logger.Info("Inquiry logs cleanup job scheduled successfully", "schedule", "1:20 AM daily")
	}

	// Payment cleanup (cron from env)
	_, err4 := u.schedulerService.AddJobWithContext(paymentCron, func(ctx context.Context) {
		if !u.postgresService.IsAvailable() {
			u.logger.Warn("Skipping Payment logs cleanup — Postgres not available")
			return
		}
		u.logger.Info("Starting scheduled Payment logs cleanup job")
		deletedCount, err := u.cleanupUsecase.CleanupOldPaymentLogs(ctx, retentionDays)
		if err != nil {
			u.logger.Error("Failed to cleanup old Payment logs", "error", err)
			return
		}
		u.logger.Info("Payment logs cleanup job completed", "deletedCount", deletedCount, "daysToKeep", retentionDays)
	})
	if err4 != nil {
		u.logger.Error("Failed to schedule Payment logs cleanup job", "error", err4)
	} else {
		u.logger.Info("Payment logs cleanup job scheduled successfully", "schedule", "1:30 AM daily")
	}

	// Schedule File logs cleanup job
	_, err6 := u.schedulerService.AddJob(fileLogCron, func() {
		u.logger.Info("Starting scheduled File logs cleanup job")
		retention := os.Getenv("FILE_LOG_RETENTION_DAYS")
		if strings.TrimSpace(retention) == "" {
			retention = "50"
		}
		retentionDays, convErr := strconv.Atoi(retention)
		if convErr != nil {
			u.logger.Error("Failed to convert FILE_LOG_RETENTION_DAYS to int", "error", convErr)
			return
		}

		deletedCount, cleanErr := service.CleanupOldLogFiles(retentionDays)
		if cleanErr != nil {
			u.logger.Error("Failed to cleanup old file logs", "error", cleanErr)
			return
		}
		u.logger.Info("File logs cleanup job completed", "deletedCount", deletedCount, "daysToKeep", retentionDays)
	})
	if err6 != nil {
		u.logger.Error("Failed to schedule File logs cleanup job", "error", err6)
	} else {
		u.logger.Info("File logs cleanup job scheduled successfully", "schedule", fileLogCron)
	}

	// Optional: partition maintenance job (ensures monthly partitions ahead)
	partitionCron := os.Getenv("PARTITION_MAINTENANCE_CRON")
	if strings.TrimSpace(partitionCron) == "" {
		partitionCron = "0 0 2 * * *" // 02:00 daily by default
	}
	_, err5 := u.schedulerService.AddJobWithContext(partitionCron, func(ctx context.Context) {
		if !u.postgresService.IsAvailable() {
			u.logger.Warn("Skipping partition maintenance — Postgres not available")
			return
		}
		u.logger.Info("Starting partition maintenance job")
		// Attempt to ensure partitions (safe no-op if parents not partitioned)
		if _, execErr := u.postgresService.Exec(ctx, "SELECT maintenance.ensure_monthly_partitions(2)"); execErr != nil {
			u.logger.Error("Partition maintenance failed", "error", execErr)
			return
		}
		u.logger.Info("Partition maintenance completed")
	})
	if err5 != nil {
		u.logger.Error("Failed to schedule partition maintenance job", "error", err5)
	} else {
		u.logger.Info("Partition maintenance job scheduled", "schedule", partitionCron)
	}

	// Daily reconciliation report export job
	reconciliationCron := os.Getenv("RECONCILIATION_EXPORT_CRON")
	if reconciliationCron == "" {
		reconciliationCron = "0 0 1 * * *" // 01:00 AM daily by default
	}
	_, err7 := u.schedulerService.AddJobWithContext(reconciliationCron, func(ctx context.Context) {
		if !u.postgresService.IsAvailable() {
			u.logger.Warn("Skipping reconciliation export — Postgres not available")
			return
		}
		u.logger.Info("Starting scheduled reconciliation report export job")

		// Get report date (yesterday by default)
		reportDate := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
		if envDate := os.Getenv("RECONCILIATION_REPORT_DATE"); envDate != "" {
			reportDate = envDate
		}

		// Query reconciliation data from database
		query := "SELECT * FROM payment.get_daily_reconciliation_report($1)"
		rows, err := u.postgresService.Query(ctx, query, reportDate)
		if err != nil {
			u.logger.Error("Failed to query reconciliation report", "error", err, "reportDate", reportDate)
			return
		}
		defer rows.Close()

		// Prepare output directory
		outputDir := os.Getenv("RECONCILIATION_OUTPUT_DIR")
		if outputDir == "" {
			outputDir = "./reports"
		}
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			u.logger.Error("Failed to create output directory", "error", err, "dir", outputDir)
			return
		}

		// Generate filename: reconcile_pps_YYYYMMDD.csv
		filename := fmt.Sprintf("reconcile_pps_%s.csv", strings.ReplaceAll(reportDate, "-", ""))
		filepath := fmt.Sprintf("%s/%s", outputDir, filename)

		// Create CSV file
		file, err := os.Create(filepath)
		if err != nil {
			u.logger.Error("Failed to create CSV file", "error", err, "filepath", filepath)
			return
		}
		defer file.Close()

		// Write CSV header
		header := "Timestamp;Ref ID;Client Number;Client Name;TARIF/DAYA;RP TOKEN;Jumlah KWH;Nomor Token;Amount;Sales Price;Partner Ref ID\n"
		if _, err := file.WriteString(header); err != nil {
			u.logger.Error("Failed to write CSV header", "error", err)
			return
		}

		// Write CSV rows
		rowCount := 0
		for rows.Next() {
			var timestamp, refID, clientNumber, clientName, tarif, rpToken, kwh, nomorToken string
			var amount, salesPrice float64
			var partnerRefID string

			if err := rows.Scan(&timestamp, &refID, &clientNumber, &clientName, &tarif, &rpToken, &kwh, &nomorToken, &amount, &salesPrice, &partnerRefID); err != nil {
				u.logger.Error("Failed to scan row", "error", err)
				continue
			}

			// Write row with semicolon delimiter
			row := fmt.Sprintf("%s;%s;%s;%s;%s;%s;%s;%s;%.2f;%.2f;%s\n",
				timestamp, refID, clientNumber, clientName, tarif, rpToken, kwh, nomorToken, amount, salesPrice, partnerRefID)
			if _, err := file.WriteString(row); err != nil {
				u.logger.Error("Failed to write CSV row", "error", err)
				continue
			}
			rowCount++
		}

		if err := rows.Err(); err != nil {
			u.logger.Error("Error iterating rows", "error", err)
			return
		}

		u.logger.Info("Reconciliation report export completed",
			"filepath", filepath,
			"reportDate", reportDate,
			"rowCount", rowCount)
	})

	if err7 != nil {
		u.logger.Error("Failed to schedule reconciliation export job", "error", err7)
	} else {
		u.logger.Info("Reconciliation export job scheduled successfully", "schedule", reconciliationCron)
	}
}
