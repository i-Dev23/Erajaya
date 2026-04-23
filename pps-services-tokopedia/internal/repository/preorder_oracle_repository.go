package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/service"
)

type PreorderOracleRepository struct {
	oracleService service.OracleService
	logger        service.Logger
	redisClient   service.RedisClient
}

func NewPreorderOracleRepository(oracleService service.OracleService, logger service.Logger, redisClient service.RedisClient) domain.PreorderRepository {
	return &PreorderOracleRepository{
		oracleService: oracleService,
		logger:        logger,
		redisClient:   redisClient,
	}
}

// Preorder calls Oracle stored procedure request2JualPreOrder
// Stored procedure signature:
// BEGIN request2JualPreOrder(
//
//	p_user=>:1, p_msisdn=>:2, p_produk=>:3, p_notrx=>:4, p_signature=>:5,
//	p_ip=>:6, p_id=>:7, p_status=>:8, p_queue_name=>:9, err=>:10, msg=>:11
//
// ); END;
func (r *PreorderOracleRepository) Preorder(ctx context.Context, req *domain.PreorderRequestDomain) (*domain.PreorderResponseDomain, error) {
	// Prepare the PL/SQL block
	query := `BEGIN request2JualPreOrder(
		p_user => :1,
		p_msisdn => :2,
		p_produk => :3,
		p_notrx => :4,
		p_signature => :5,
		p_ip => :6,
		p_id => :7,
		p_status => :8,
		p_queue_name => :9,
		err => :10,
		msg => :11
	); END;`

	// Handle signature parameter - use NULL if empty to avoid conversion errors
	var signature interface{}
	if req.Signature == "" {
		signature = nil // Send NULL to Oracle
	} else {
		signature = req.Signature
	}

	// Handle IP address - default if empty
	ipAddr := req.Addr
	if ipAddr == "" {
		ipAddr = "0.0.0.0"
	}

	r.logger.Info("Calling request2JualPreOrder stored procedure",
		"user", req.User,
		"msisdn", req.MDN,
		"product", req.Product,
		"notrx", req.NoTrx,
		"signature", req.Signature,
		"ip", ipAddr)

	// Prepare output parameters with pre-allocated buffers for go-ora
	serverId := strings.Repeat(" ", 4000)
	status := strings.Repeat(" ", 4000)
	queueName := strings.Repeat(" ", 4000) // Buffer for p_queue_name
	var errCode int64                      // Error code
	errMsg := strings.Repeat(" ", 4000)    // Buffer for error message

	// Execute the stored procedure
	_, err := r.oracleService.Exec(ctx, query,
		// Input parameters
		req.User,    // :1 p_user
		req.MDN,     // :2 p_msisdn
		req.Product, // :3 p_produk
		req.NoTrx,   // :4 p_notrx
		signature,   // :5 p_signature - NULL if empty
		ipAddr,      // :6 p_ip
		// Output parameters
		sql.Out{Dest: &serverId},  // :7 p_id (OUT) - Server/Message ID
		sql.Out{Dest: &status},    // :8 p_status (OUT) - Status
		sql.Out{Dest: &queueName}, // :9 p_queue_name (OUT) - Queue name
		sql.Out{Dest: &errCode},   // :10 err (OUT) - Error code
		sql.Out{Dest: &errMsg},    // :11 msg (OUT) - Error message
	)

	if err != nil {
		r.logger.Error("Failed to execute request2JualPreOrder",
			"error", err,
			"user", req.User,
			"msisdn", req.MDN,
			"product", req.Product)
		return nil, fmt.Errorf("failed to execute request2JualPreOrder: %w", err)
	}

	// Convert byte arrays to strings and trim null bytes
	serverIdStr := string(serverId)
	serverIdStr = trimNullBytes(serverIdStr)

	statusStr := string(status)
	statusStr = trimNullBytes(statusStr)

	queueNameStr := string(queueName)
	queueNameStr = trimNullBytes(queueNameStr)

	errMsgStr := string(errMsg)
	errMsgStr = trimNullBytes(errMsgStr)

	// Build response
	response := &domain.PreorderResponseDomain{
		ServerId:   serverIdStr,
		Status:     statusStr,
		QueueName:  queueNameStr,
		OuterRCode: errCode,
		OuterRMsg:  errMsgStr,
	}

	r.logger.Info("request2JualPreOrder executed successfully",
		"serverId", response.ServerId,
		"status", response.Status,
		"queueName", response.QueueName,
		"errorCode", response.OuterRCode,
		"errorMsg", response.OuterRMsg)

	// Check if there was an error from the stored procedure
	if errCode != 0 {
		r.logger.Warn("request2JualPreOrder returned error",
			"errorCode", errCode,
			"errorMsg", errMsgStr)
	}

	return response, nil
}

// UpdatePreorderStatus updates the preorder status
// You can implement this later if you have the stored procedure for updating status
func (r *PreorderOracleRepository) UpdatePreorderStatus(ctx context.Context, msgid string, status string, message string) (*domain.UpdatePreorderStatusResponseDomain, error) {
	// Example: BEGIN update_preorder_status(p_msgid=>:1, p_status=>:2, p_message=>:3, err=>:4, msg=>:5); END;
	query := `BEGIN MSG.updPreOrderPublish(InMsgId=>:1,InPublishStatus=>:2,InPublishMessage=>:3,OutError=>:4,OutMessage=>:5); END;`
	var errCode int64                   // Error code
	errMsg := strings.Repeat(" ", 4000) // Buffer for error message

	_, err := r.oracleService.Exec(ctx, query,
		msgid,
		status,
		message,
		sql.Out{Dest: &errCode}, // :4 OutError
		sql.Out{Dest: &errMsg},  // :5 OutMessage
	)

	if err != nil {
		r.logger.Error("Failed to update preorder status",
			"error", err,
			"msgid", msgid,
			"status", status,
			"message", message)
		return nil, fmt.Errorf("failed to update preorder status: %w", err)
	}
	r.logger.Info("UpdatePreorderStatus called",
		"msgid", msgid,
		"status", status,
		"message", message)

	// Return success for now
	return &domain.UpdatePreorderStatusResponseDomain{
		OuterRCode: 0,
		OuterRMsg:  "Success",
	}, nil
}

// trimNullBytes removes null bytes and trailing spaces from string
func trimNullBytes(s string) string {
	// Remove null bytes
	s = strings.ReplaceAll(s, "\x00", "")
	// Trim spaces
	return strings.TrimSpace(s)
}
