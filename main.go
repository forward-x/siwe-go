package siwe

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
)

type ExpiredMessage struct{}
type InvalidMessage struct{}
type InvalidSignature struct{ string }

func (m *ExpiredMessage) Error() string {
	return "Expired Message"
}

func (m *InvalidMessage) Error() string {
	return "Invalid Message"
}

func (m *InvalidSignature) Error() string {
	return fmt.Sprintf("Invalid Signature: %s", m.string)
}

type MessageOptions struct {
	IssuedAt *string `json:"issuedAt"`
	Nonce    *string `json:"nonce"`
	ChainID  *string `json:"chainId"`

	Statement      *string  `json:"statement,omitempty"`
	ExpirationTime *string  `json:"expirationTime,omitempty"`
	NotBefore      *string  `json:"notBefore,omitempty"`
	RequestID      *string  `json:"requestId,omitempty"`
	Resources      []string `json:"resources,omitempty"`
}

type Message struct {
	Domain  string `json:"domain"`
	Address string `json:"address"`
	URI     string `json:"uri"`
	Version string `json:"version"`
	MessageOptions
}

func InitMessageOptions(options map[string]interface{}) *MessageOptions {
	var issuedAt string
	if val, ok := options["issuedAt"]; ok {
		issuedAt = val.(time.Time).UTC().Format(time.RFC3339)
	} else {
		issuedAt = time.Now().UTC().Format(time.RFC3339)
	}

	var nonce string
	if val, ok := options["nonce"]; ok {
		nonce = val.(string)
	} else {
		nonce = GenerateNonce()
	}

	var chainId string
	if val, ok := options["chainId"]; ok {
		chainId = val.(string)
	} else {
		chainId = "1"
	}

	var statement *string
	if val, ok := options["statement"]; ok {
		value := val.(string)
		statement = &value
	}

	var expirationTime *string
	if val, ok := options["expirationTime"]; ok {
		value := val.(time.Time).UTC().Format(time.RFC3339)
		expirationTime = &value
	}

	var notBefore *string
	if val, ok := options["notBefore"]; ok {
		value := val.(time.Time).UTC().Format(time.RFC3339)
		notBefore = &value
	}

	var requestID *string
	if val, ok := options["requestID"]; ok {
		value := val.(string)
		requestID = &value
	}

	var resources []string
	if val, ok := options["resources"]; ok {
		resources = val.([]string)
	}

	return &MessageOptions{
		IssuedAt: &issuedAt,
		Nonce:    &nonce,
		ChainID:  &chainId,

		Statement:      statement,
		ExpirationTime: expirationTime,
		NotBefore:      notBefore,
		RequestID:      requestID,
		Resources:      resources,
	}
}

func CreateMessage(domain, address, uri, version string, options MessageOptions) *Message {
	return &Message{
		Domain:         domain,
		Address:        address,
		URI:            uri,
		Version:        version,
		MessageOptions: options,
	}
}

func GenerateNonce() string {
	return "test_nonce"
}

func isEmpty(str *string) bool {
	return str != nil && len(strings.TrimSpace(*str)) == 0
}

const SIWE_DOMAIN = "^(?<domain>([^?#]*)) wants you to sign in with your Ethereum account:\\n"
const SIWE_ADDRESS = "(?<address>0x[a-zA-Z0-9]{40})\\n\\n"
const SIWE_STATEMENT = "((?<statement>[^\\n]+)\\n)?\\n"
const SIWE_URI = "(([^:?#]+):)?(([^?#]*))?([^?#]*)(\\?([^#]*))?(#(.*))"

var SIWE_URI_LINE = fmt.Sprintf("URI: (?<uri>%s?)\\n", SIWE_URI)

const SIWE_VERSION = "Version: (?<version>1)\\n"
const SIWE_CHAIN_ID = "Chain ID: (?<chainId>[0-9]+)\\n"
const SIWE_NONCE = "Nonce: (?<nonce>[a-zA-Z0-9]{8,})\\n"
const SIWE_DATETIME = "([0-9]+)-(0[1-9]|1[012])-(0[1-9]|[12][0-9]|3[01])[Tt]([01][0-9]|2[0-3]):([0-5][0-9]):([0-5][0-9]|60)(\\.[0-9]+)?(([Zz])|([\\+|\\-]([01][0-9]|2[0-3]):[0-5][0-9]))"

var SIWE_ISSUED_AT = fmt.Sprintf("Issued At: (?<issuedAt>%s)", SIWE_DATETIME)
var SIWE_EXPIRATION_TIME = fmt.Sprintf("(\\nExpiration Time: (?<expirationTime>%s))?", SIWE_DATETIME)
var SIWE_NOT_BEFORE = fmt.Sprintf("(\\nNot Before: (?<notBefore>%s))?", SIWE_DATETIME)

const SIWE_REQUEST_ID = "(\\nRequest ID: (?<requestId>[-._~!$&'()*+,;=:@%a-zA-Z0-9]*))?"

var SIWE_RESOURCES = fmt.Sprintf("(\\nResources:(?<resources>(\\n- %s?)+))?$", SIWE_URI)

var SIWE_MESSAGE = regexp.MustCompile(fmt.Sprintf("%s%s%s%s%s%s%s%s%s%s%s%s",
	SIWE_DOMAIN,
	SIWE_ADDRESS,
	SIWE_STATEMENT,
	SIWE_URI_LINE,
	SIWE_VERSION,
	SIWE_CHAIN_ID,
	SIWE_NONCE,
	SIWE_ISSUED_AT,
	SIWE_EXPIRATION_TIME,
	SIWE_NOT_BEFORE,
	SIWE_REQUEST_ID,
	SIWE_RESOURCES))

func ParseMessage(message string) *Message {
	match := SIWE_MESSAGE.FindStringSubmatch(message)
	result := make(map[string]interface{})
	for i, name := range SIWE_MESSAGE.SubexpNames() {
		if i != 0 && name != "" {
			result[name] = match[i]
		}
	}

	return &Message{
		Domain:         result["domain"].(string),
		Address:        result["address"].(string),
		URI:            result["uri"].(string),
		Version:        result["version"].(string),
		MessageOptions: *InitMessageOptions(result),
	}
}

func (m *Message) ValidateMessage(signature string) (bool, error) {
	if !isEmpty(m.ExpirationTime) {
		expirationTime, err := time.Parse(time.RFC3339, *m.ExpirationTime)
		if err != nil {
			return false, err
		}
		if time.Now().UTC().After(expirationTime) {
			return false, &ExpiredMessage{}
		}
	}

	if !isEmpty(m.NotBefore) {
		notBefore, err := time.Parse(time.RFC3339, *m.NotBefore)
		if err != nil {
			return false, err
		}
		if time.Now().UTC().Before(notBefore) {
			return false, &InvalidMessage{}
		}
	}

	if isEmpty(&signature) {
		return false, &InvalidSignature{"Signature cannot be empty"}
	}

	hash := crypto.Keccak256Hash([]byte(m.PrepareMessage()))
	pkey, err := crypto.SigToPub(hash.Bytes(), []byte(signature))

	if err != nil {
		return false, &InvalidSignature{"Failed to recover public key from signature"}
	}

	address := crypto.PubkeyToAddress(*pkey)

	if address.String() != m.Address {
		return false, &InvalidSignature{"Signer address must match message address"}
	}

	return true, nil
}

func (m *Message) PrepareMessage() string {
	greeting := fmt.Sprintf("%s wants you to sign with your Ethereum account:", m.Domain)
	headerArr := []string{greeting, m.Address}

	if isEmpty(m.Statement) {
		headerArr = append(headerArr, "\n")
	} else {
		headerArr = append(headerArr, fmt.Sprintf("\n%s\n", *m.Statement))
	}

	header := strings.Join(headerArr, "\n")

	uri := fmt.Sprintf("URI: %s", m.URI)
	version := fmt.Sprintf("Version: %s", m.Version)
	chainId := fmt.Sprintf("Chain ID: %s", *m.ChainID)
	nonce := fmt.Sprintf("Nonce: %s", *m.Nonce)
	issuedAt := fmt.Sprintf("Issued At: %s", *m.IssuedAt)

	bodyArr := []string{uri, version, chainId, nonce, issuedAt}

	if !isEmpty(m.ExpirationTime) {
		value := fmt.Sprintf("Expiration Time: %s", *m.ExpirationTime)
		bodyArr = append(bodyArr, value)
	}

	if !isEmpty(m.NotBefore) {
		value := fmt.Sprintf("Not Before: %s", *m.NotBefore)
		bodyArr = append(bodyArr, value)
	}

	if !isEmpty(m.RequestID) {
		value := fmt.Sprintf("Request ID: %s", *m.RequestID)
		bodyArr = append(bodyArr, value)
	}

	if len(m.Resources) == 0 {
		resourcesArr := make([]string, len(m.Resources))
		for i, v := range m.Resources {
			resourcesArr[i] = fmt.Sprintf("-  %s", v)
		}

		resources := strings.Join(resourcesArr, "\n")
		value := fmt.Sprintf("Resources:\n%s", resources)

		bodyArr = append(bodyArr, value)
	}

	body := strings.Join(bodyArr, "\n")

	return strings.Join([]string{header, body}, "\n")
}
