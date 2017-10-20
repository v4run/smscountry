package smscountry

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Defines the different endpoints in SMS country
const (
	SMSCountryScheme = `https`
	SMSCountryHost   = `api.smscountry.com`
	MessagePath      = `/SMSCwebservice_bulk.aspx`
	BulMessagePath   = `/SMSCwebservice_bulk.aspx`
	MultiMessagePath = `/SMSCWebservice_MultiMessages.asp`
	BulkReportPath   = `/smscwebservices_bulk_reports.aspx`
	BalancePath      = `/SMSCwebservice_User_GetBal.asp`
)

// Defines the keys in requests
const (
	User           = "User"
	Password       = "passwd"
	SenderID       = "sid"
	MobileNumber   = "mobilenumber"
	Message        = "message"
	MessageType    = "mtype"
	DeliveryReport = "dr"
	MultiMessage   = "mno_msg"
)

// Delivery report values
const (
	SendDeliveryReport     = "Y"
	DontSendDeliveryReport = "N"
)

// Message types
const (
	NormalMessage  = "N"
	UnicodeMessage = "OL"
	PictureMessage = "P"
	Logo           = "L"
	FlashMessage   = "F"
	WAPPush        = "WP" // Specify the wap url in the parameter wap_url
	LongSMS        = "LS" // GPRS based
	Ringtone       = "R"
)

// Defines the different errors
var (
	ErrEmptyResponse = errors.New("Empty response from server")
)

// Client defines a sms country client
type Client struct {
	User              string
	Password          string
	balanceEnquiryURL string
	httpClient        *http.Client
}

// Balance returns the balance available for the user
func (s Client) Balance() (bal float64, err error) {
	if resp, er := s.httpClient.Get(s.balanceEnquiryURL); er != nil {
		err = er
	} else {
		if resp.Body == nil {
			err = ErrEmptyResponse
		}
		defer func(e *error) {
			if err := resp.Body.Close(); err != nil {
				*e = err
			}
		}(&err)
		if r, er := ioutil.ReadAll(resp.Body); er != nil {
			err = er
		} else {
			if v, er := strconv.ParseFloat(strings.SplitN(string(r), " ", 2)[0], 64); er != nil {
				err = er
			} else {
				bal = v
			}
		}
	}
	return bal, err
}

// NewSender returns a new sender
func (s Client) NewSender(senderID string) *Sender {
	return &Sender{
		Client:   s,
		SenderID: senderID,
	}
}

// Sender defines a sender
type Sender struct {
	Client   Client
	SenderID string
}

func (s *Sender) sendMessage(path string, content io.Reader) (err error) {
	if resp, er := s.Client.httpClient.Post((&url.URL{
		Host:   SMSCountryHost,
		Path:   path,
		Scheme: SMSCountryScheme,
	}).String(), "application/x-www-form-urlencoded", content); er != nil {
		err = er
	} else {
		if resp.Body == nil {
			return ErrEmptyResponse
		}
		defer func(e *error) {
			if err := resp.Body.Close(); err != nil {
				if e == nil || *e == nil {
					*e = err
				} else {
					*e = fmt.Errorf("Error: %v, Body close error: %v", *e, err)
				}
			}
		}(&err)
		if r, er := ioutil.ReadAll(resp.Body); er != nil {
			err = er
		} else {
			response := strings.TrimSpace(string(r))
			if !strings.HasPrefix(response, "OK:") && response != "SMS message(s) sent" {
				return fmt.Errorf("Error sending SMS. Response: %s", response)
			}
		}
	}
	return nil
}

// SendSMS sends an SMS to the recipient
func (s *Sender) SendSMS(message, mobileNumber string, deliveryReport bool) (err error) {
	query := url.Values{}
	query.Add(User, s.Client.User)
	query.Add(Password, s.Client.Password)
	query.Add(SenderID, s.SenderID)
	query.Add(MobileNumber, mobileNumber)
	query.Add(Message, message)
	query.Add(MessageType, NormalMessage)
	if deliveryReport {
		query.Add(DeliveryReport, SendDeliveryReport)
	} else {
		query.Add(DeliveryReport, DontSendDeliveryReport)
	}
	return s.sendMessage(MessagePath, strings.NewReader(query.Encode()))
}

// SendBulkSMS sends an SMS to the recipient
func (s *Sender) SendBulkSMS(messages, mobileNumbers []string, deliveryReport bool) (err error) {
	msgBuf := new(bytes.Buffer)
	msgBuf.WriteString(fmt.Sprintf("%s^%s", mobileNumbers[0], messages[0]))
	for i := 1; i < len(messages); i++ {
		msgBuf.WriteString(fmt.Sprintf("~%s^%s", mobileNumbers[i], messages[i]))
	}
	query := url.Values{}
	query.Add(User, s.Client.User)
	query.Add(Password, s.Client.Password)
	query.Add(SenderID, s.SenderID)
	query.Add(MultiMessage, msgBuf.String())
	query.Add(MessageType, NormalMessage)
	return s.sendMessage(MultiMessagePath, strings.NewReader(query.Encode()))
}

// New returns a new instance of Client
func New(user, password string) *Client {
	s := &Client{
		User:     user,
		Password: password,
		balanceEnquiryURL: (&url.URL{
			Host:   SMSCountryHost,
			Path:   BalancePath,
			Scheme: SMSCountryScheme,
			RawQuery: url.Values{
				"User":   {user},
				"passwd": {password},
			}.Encode(),
		}).String(),
		httpClient: &http.Client{Timeout: time.Duration(time.Minute)},
	}
	return s
}
