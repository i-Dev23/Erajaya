package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"pps-services-consumer/constanta"
	"pps-services-consumer/database"
	"pps-services-consumer/model"
	"pps-services-consumer/util"
	log "pps-services-consumer/util"
	"regexp"
	"strconv"
	"strings"

	go_ora "github.com/sijms/go-ora/v2"
)

func logOracleExecError(op string, err error) {
	if err == nil {
		return
	}
	log.Printf("%s: oracle exec error type=%T err=%v", op, err, err)

	// Extract ORA error codes from the error string (works across drivers/wrappers).
	re := regexp.MustCompile(`ORA-\d{5}`)
	codes := re.FindAllString(err.Error(), -1)
	if len(codes) > 0 {
		log.Printf("%s: oracle err codes=%s", op, strings.Join(codes, ","))
	}

	// Log unwrap chain to reveal wrapped driver errors.
	for unwrap := errors.Unwrap(err); unwrap != nil; unwrap = errors.Unwrap(unwrap) {
		log.Printf("%s: oracle exec error wrapped type=%T err=%v", op, unwrap, unwrap)
	}
}

func initConnection(db string) (*sql.DB, error) {
	var err error
	var Conn *sql.DB

	Conn, err = sql.Open("oracle", db)
	if err != nil {
		util.ComposeMessageTelegramNotification(err.Error())
		fmt.Println("Can't open the driver: ", err)
		return nil, err
	}

	err = Conn.Ping()
	if err != nil {
		util.ComposeMessageTelegramNotification(err.Error())
		fmt.Println("Can't ping connection: ", err)
		return nil, err
	}
	fmt.Println("Successfully connected to database.")

	return Conn, nil
}

// isConnectionError checks if the error is a connection-level error that requires pool reset.
func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "connection reset by peer") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "EOF") ||
		strings.Contains(msg, "driver: bad connection")
}

// getConnectionWithRetry gets a connection, resets pool and retries once on connection error.
func getConnectionWithRetry() (*sql.DB, error) {
	conn, err := database.GetConnection()
	if err != nil {
		// First attempt failed, reset and retry
		database.ResetPool()
		conn, err = database.GetConnection()
	}
	return conn, err
}

func UpdPreOrderConsume(msgid string, status string) model.PreOrderConsumeResult {
	var result model.PreOrderConsumeResult
	var OutMessage string
	var OutError int64
	var OutImsi string
	var OutRemarkImsi string
	var OutMid string
	var OutStoreId string
	var OutQueueName string
	var OutTypeVoucher string
	var OutProvider string
	var OutCommand string

	OutMessage = strings.Repeat(" ", 4000)
	OutImsi = strings.Repeat(" ", 200)
	OutRemarkImsi = strings.Repeat(" ", 200)
	OutMid = strings.Repeat(" ", 200)
	OutQueueName = strings.Repeat(" ", 500)
	OutTypeVoucher = strings.Repeat(" ", 200)
	OutProvider = strings.Repeat(" ", 200)
	OutCommand = strings.Repeat(" ", 200)
	OutStoreId = strings.Repeat(" ", 50)

	msgidRaw := msgid
	msgidTrim := strings.TrimSpace(msgid)
	statusTrim := strings.TrimSpace(status)
	msgidParsed, msgidParseErr := strconv.ParseInt(msgidTrim, 10, 64)
	if msgidParseErr != nil {
		// We still proceed to call SP (to preserve existing behavior), but log enough context
		// to identify invalid/non-numeric msgid quickly.
		log.Printf(
			"UpdPreOrderConsume input warning: msgid is not a valid int64 msgid_raw=%q msgid_trim=%q status=%q parse_err=%v",
			msgidRaw,
			msgidTrim,
			statusTrim,
			msgidParseErr,
		)
	}

	log.Printf(
		"UpdPreOrderConsume call: msgid_raw=%q msgid_trim=%q msgid_len=%d msgid_parsed=%d status=%q",
		msgidRaw,
		msgidTrim,
		len(msgidRaw),
		msgidParsed,
		statusTrim,
	)

	conn, errInit := getConnectionWithRetry()

	if errInit != nil {
		log.Println("errInit => " + errInit.Error())
		result.OutError = 1
		return result
	}

	_, err := conn.Exec(`BEGIN MSG.updPreOrderConsume(InMsgId=>:1,InConsumeStatus=>:2,OutImsi=>:3,OutRemarkImsi=>:4,OutMid=>:5,OutStoreId=>:6,OutQueueName=>:7,OutTypeVoucher=>:8,OutProvider=>:9,OutCommand=>:10,OutError=>:11,OutMessage=>:12); END;`,
		msgid,
		status,
		go_ora.Out{Dest: &OutImsi},
		go_ora.Out{Dest: &OutRemarkImsi},
		go_ora.Out{Dest: &OutMid},
		go_ora.Out{Dest: &OutStoreId},
		go_ora.Out{Dest: &OutQueueName},
		go_ora.Out{Dest: &OutTypeVoucher},
		go_ora.Out{Dest: &OutProvider},
		go_ora.Out{Dest: &OutCommand},
		go_ora.Out{Dest: &OutError},
		go_ora.Out{Dest: &OutMessage})
	if err != nil {
		if isConnectionError(err) {
			database.ResetPool()
		}
		util.ComposeMessageTelegramNotification(err.Error())
		log.Printf(
			"error Exec updPreOrderConsume: msgid_raw=%q msgid_trim=%q msgid_parsed=%d status=%q err=%v",
			msgidRaw,
			msgidTrim,
			msgidParsed,
			statusTrim,
			err,
		)
		logOracleExecError("UpdPreOrderConsume", err)
		result.OutError = 1
		return result
	}

	result.OutError = OutError
	result.OutMessage = strings.TrimSpace(OutMessage)
	result.OutImsi = strings.TrimSpace(OutImsi)
	result.OutRemarkImsi = strings.TrimSpace(OutRemarkImsi)
	result.OutMid = strings.TrimSpace(OutMid)
	result.OutStoreId = strings.TrimSpace(OutStoreId)
	result.OutQueueName = strings.TrimSpace(OutQueueName)
	result.OutTypeVoucher = strings.TrimSpace(OutTypeVoucher)
	result.OutProvider = strings.TrimSpace(OutProvider)
	result.OutCommand = strings.TrimSpace(OutCommand)
	result.OutProvider = strings.TrimSpace(OutProvider)

	log.Printf(
		"UpdPreOrderConsume ok: outError=%d outMessage_len=%d outMessage=%q outStoreId=%q outQueueName_len=%d outQueueName=%q outTypeVoucher=%q outProvider=%q outCommand=%q outRemarkImsi_len=%d",
		result.OutError,
		len(result.OutMessage),
		result.OutMessage,
		result.OutStoreId,
		len(result.OutQueueName),
		result.OutQueueName,
		result.OutTypeVoucher,
		result.OutProvider,
		result.OutCommand,
		len(result.OutRemarkImsi),
	)

	return result
}

func SellWithId(request model.SellRequestModel) model.BaseH2hResponse {
	var response model.BaseH2hResponse

	var OutMessage string
	var OutError int64
	var ServerIDTrx string
	var Status string

	ServerIDTrx = strings.Repeat(" ", 4000)
	OutMessage = strings.Repeat(" ", 4000)
	Status = strings.Repeat(" ", 1000)

	conn, errInit := getConnectionWithRetry()

	if errInit != nil {
		response.Status = constanta.FAILED_STATUS
		response.Message = errInit.Error()
		strID := fmt.Sprintf("%d", request.Msgid)
		param_request := "p_msg_id = " + strID + " || " + "p_user = " + request.User + " || " + "p_msisdn = " + request.MDN + " || " + "p_produk = " + request.Produk + " || " + "p_notrx = " + request.NoTrx
		log.Println("error initConnection request2JualRandomWithID => " + param_request + " " + errInit.Error())
	} else {
		_, err := conn.Exec(`BEGIN MSG.request2JualRandomWithID(p_msg_id=>:1,p_user=>:2,p_msisdn=>:3,p_produk=>:4,p_notrx=>:5,p_signature=>:6,p_ip=>:7,p_id=>:8,p_status=>:9,err=>:10,msg=>:11); END;`,
			request.Msgid,
			request.User,
			request.MDN,
			request.Produk,
			request.NoTrx,
			request.Signature,
			request.Addr,
			go_ora.Out{Dest: &ServerIDTrx},
			go_ora.Out{Dest: &Status},
			go_ora.Out{Dest: &OutError},
			go_ora.Out{Dest: &OutMessage})

		if err != nil {
			if isConnectionError(err) {
				database.ResetPool()
			}
			strID := fmt.Sprintf("%d", request.Msgid)
			param_request := "p_msg_id = " + strID + " || " + "p_user = " + request.User + " || " + "p_msisdn = " + request.MDN + " || " + "p_produk = " + request.Produk + " || " + "p_notrx = " + request.NoTrx
			util.ComposeMessageTelegramNotification(param_request + " " + err.Error())
			response.Status = constanta.FAILED_STATUS
			response.Message = err.Error()
			response.ClientNoTrx = request.NoTrx
			log.Println("error exec request2JualRandomWithID => " + param_request + " " + err.Error())
		} else {
			response.Status = Status

			if "<nil>" == ServerIDTrx {
				response.ServerIDTrx = ""
			} else {
				response.ServerIDTrx = ServerIDTrx
			}

			if "<nil>" == OutMessage {
				response.Message = ""
			} else {
				response.Message = OutMessage
			}

			response.ClientNoTrx = request.NoTrx
			log.Println("sukses exec request2JualRandomWithID => " + OutMessage)
		}
	}

	return response
}

// SetTransactionStatus memanggil SP MSG.SETTRANSACTIONSTATUS untuk update status final transaksi.
// Digunakan untuk memproses pesan PROVIDER dari RabbitMQ.
// Pola error handling sama dengan UpdPreOrderConsume: reset pool + retry 1x pada connection error.
func SetTransactionStatus(event model.ProviderMessage) (int64, string) {
	var OutID int
	var OutError int64
	var OutMessage string
	OutMessage = strings.Repeat(" ", 4000)

	conn, errInit := getConnectionWithRetry()
	if errInit != nil {
		log.Println("errInit SetTransactionStatus => " + errInit.Error())
		return 1, errInit.Error()
	}

	_, err := conn.Exec(`BEGIN MSG.SETTRANSACTIONSTATUS(
		INMSGID                    => :1,
		INSTATUSTOBE               => :2,
		INSERIALNUMBER             => :3,
		INNUMBERBUYER              => :4,
		INNOMINAL                  => :5,
		INORIGINALCONVERSATIONID   => :6,
		INCONVERSATIONID           => :7,
		INMESAGETOCUSTOMER         => :8,
		INADDITIONALMESSAGE        => :9,
		OUTID                      => :10,
		OUTERROR                   => :11,
		OUTMESSAGE                 => :12
	); END;`,
		event.MsgId,
		event.StatusToBe,
		event.SerialNumber,
		event.ClientNumber,
		event.Nominal,
		event.OriginalConversationID,
		event.ConversationID,
		event.MessageToCustomer,
		event.AdditionalMessage,
		go_ora.Out{Dest: &OutID},
		go_ora.Out{Dest: &OutError},
		go_ora.Out{Dest: &OutMessage, Size: 4000})

	if err != nil {
		if isConnectionError(err) {
			database.ResetPool()
			// Retry sekali
			conn2, errRetry := getConnectionWithRetry()
			if errRetry == nil {
				_, err2 := conn2.Exec(`BEGIN MSG.SETTRANSACTIONSTATUS(
					INMSGID                    => :1,
					INSTATUSTOBE               => :2,
					INSERIALNUMBER             => :3,
					INNUMBERBUYER              => :4,
					INNOMINAL                  => :5,
					INORIGINALCONVERSATIONID   => :6,
					INCONVERSATIONID           => :7,
					INMESAGETOCUSTOMER         => :8,
					INADDITIONALMESSAGE        => :9,
					OUTID                      => :10,
					OUTERROR                   => :11,
					OUTMESSAGE                 => :12
				); END;`,
					event.MsgId,
					event.StatusToBe,
					event.SerialNumber,
					event.ClientNumber,
					event.Nominal,
					event.OriginalConversationID,
					event.ConversationID,
					event.MessageToCustomer,
					event.AdditionalMessage,
					go_ora.Out{Dest: &OutID},
					go_ora.Out{Dest: &OutError},
					go_ora.Out{Dest: &OutMessage, Size: 4000})
				if err2 == nil {
					log.Printf("SetTransactionStatus retry success, msg_id: %d, OutError: %d", event.MsgId, OutError)
					if OutError != 0 {
						log.Printf("WARNING SetTransactionStatus SP error, msg_id: %d, OutError: %d, OutMessage: %s",
							event.MsgId, OutError, strings.TrimSpace(OutMessage))
					}
					return OutError, strings.TrimSpace(OutMessage)
				}
			}
		}
		errMsg := fmt.Sprintf("error Exec SetTransactionStatus msg_id: %d => %s", event.MsgId, err.Error())
		util.ComposeMessageTelegramNotification(errMsg)
		log.Println(errMsg)
		return 1, err.Error()
	}

	if OutError != 0 {
		log.Printf("WARNING SetTransactionStatus SP error, msg_id: %d, OutError: %d, OutMessage: %s",
			event.MsgId, OutError, strings.TrimSpace(OutMessage))
	} else {
		log.Printf("SetTransactionStatus success, msg_id: %d, OutID: %d", event.MsgId, OutID)
	}

	return OutError, strings.TrimSpace(OutMessage)
}
