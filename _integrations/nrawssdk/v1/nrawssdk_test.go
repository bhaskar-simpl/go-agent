package nrawssdk

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/lambda"
	newrelic "github.com/newrelic/go-agent"
	"github.com/newrelic/go-agent/internal"
)

func testApp(t *testing.T) newrelic.Application {
	cfg := newrelic.NewConfig("appname", "0123456789012345678901234567890123456789")
	cfg.Enabled = false
	app, err := newrelic.NewApplication(cfg)
	if nil != err {
		t.Fatal(err)
	}
	internal.HarvestTesting(app, nil)
	return app
}

type fakeTransport struct{}

func (t fakeTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return &http.Response{
		Status:     "200 OK",
		StatusCode: 200,
		Body:       ioutil.NopCloser(bytes.NewReader([]byte(""))),
	}, nil
}

func newSession() *session.Session {
	r := "us-west-2"
	ses := session.New()
	ses.Config.HTTPClient.Transport = &fakeTransport{}
	ses.Config.Region = &r
	return ses
}

func TestInstrumentRequestExternal(t *testing.T) {
	app := testApp(t)
	txn := app.StartTransaction("lambda-txn", nil, nil)

	client := lambda.New(newSession())
	input := &lambda.InvokeInput{
		ClientContext:  aws.String("MyApp"),
		FunctionName:   aws.String("non-existent-function"),
		InvocationType: aws.String("Event"),
		LogType:        aws.String("Tail"),
		Payload:        []byte("{}"),
	}

	req, out := client.InvokeRequest(input)
	req = InstrumentRequest(req, txn)

	err := req.Send()
	if nil != err {
		t.Error(err)
	}
	if 200 != *out.StatusCode {
		t.Error("wrong status code on response", out.StatusCode)
	}

	txn.End()

	app.(internal.Expect).ExpectMetrics(t, []internal.WantMetric{
		{"External/all", "", true, nil},
		{"External/allOther", "", true, nil},
		{"External/lambda.us-west-2.amazonaws.com/all", "", false, nil},
		{"External/lambda.us-west-2.amazonaws.com/all", "OtherTransaction/Go/lambda-txn", false, nil},
		{"OtherTransaction/Go/lambda-txn", "", true, nil},
		{"OtherTransaction/all", "", true, nil},
	})
}

func TestInstrumentRequestDatastore(t *testing.T) {
	app := testApp(t)
	txn := app.StartTransaction("dynamodb-txn", nil, nil)

	client := dynamodb.New(newSession())
	input := &dynamodb.DescribeTableInput{
		TableName: aws.String("thebesttable"),
	}

	req, _ := client.DescribeTableRequest(input)
	req = InstrumentRequest(req, txn)

	err := req.Send()
	if nil != err {
		t.Error(err)
	}

	txn.End()

	app.(internal.Expect).ExpectMetrics(t, []internal.WantMetric{
		{"Datastore/DynamoDB/all", "", true, nil},
		{"Datastore/DynamoDB/allOther", "", true, nil},
		{"Datastore/all", "", true, nil},
		{"Datastore/allOther", "", true, nil},
		{"Datastore/instance/DynamoDB/dynamodb.us-west-2.amazonaws.com/unknown", "", false, nil},
		{"Datastore/operation/DynamoDB/DescribeTable", "", false, nil},
		{"Datastore/statement/DynamoDB/thebesttable/DescribeTable", "", false, nil},
		{"Datastore/statement/DynamoDB/thebesttable/DescribeTable", "OtherTransaction/Go/dynamodb-txn", false, nil},
		{"OtherTransaction/Go/dynamodb-txn", "", true, nil},
		{"OtherTransaction/all", "", true, nil},
	})
}

func TestInstrumentRequestExternalNoTxn(t *testing.T) {
	client := lambda.New(newSession())
	input := &lambda.InvokeInput{
		ClientContext:  aws.String("MyApp"),
		FunctionName:   aws.String("non-existent-function"),
		InvocationType: aws.String("Event"),
		LogType:        aws.String("Tail"),
		Payload:        []byte("{}"),
	}

	req, out := client.InvokeRequest(input)
	req = InstrumentRequest(req, nil)

	err := req.Send()
	if nil != err {
		t.Error(err)
	}
	if 200 != *out.StatusCode {
		t.Error("wrong status code on response", out.StatusCode)
	}
}

func TestInstrumentRequestDatastoreNoTxn(t *testing.T) {
	client := dynamodb.New(newSession())
	input := &dynamodb.DescribeTableInput{
		TableName: aws.String("thebesttable"),
	}

	req, _ := client.DescribeTableRequest(input)
	req = InstrumentRequest(req, nil)

	err := req.Send()
	if nil != err {
		t.Error(err)
	}
}

func TestInstrumentSessionExternal(t *testing.T) {
	app := testApp(t)
	txn := app.StartTransaction("lambda-txn", nil, nil)

	ses := newSession()
	ses = InstrumentSession(ses)
	client := lambda.New(ses)

	input := &lambda.InvokeInput{
		ClientContext:  aws.String("MyApp"),
		FunctionName:   aws.String("non-existent-function"),
		InvocationType: aws.String("Event"),
		LogType:        aws.String("Tail"),
		Payload:        []byte("{}"),
	}

	req, out := client.InvokeRequest(input)
	req.HTTPRequest = newrelic.RequestWithTransactionContext(req.HTTPRequest, txn)

	err := req.Send()
	if nil != err {
		t.Error(err)
	}
	if 200 != *out.StatusCode {
		t.Error("wrong status code on response", out.StatusCode)
	}

	txn.End()

	app.(internal.Expect).ExpectMetrics(t, []internal.WantMetric{
		{"External/all", "", true, nil},
		{"External/allOther", "", true, nil},
		{"External/lambda.us-west-2.amazonaws.com/all", "", false, nil},
		{"External/lambda.us-west-2.amazonaws.com/all", "OtherTransaction/Go/lambda-txn", false, nil},
		{"OtherTransaction/Go/lambda-txn", "", true, nil},
		{"OtherTransaction/all", "", true, nil},
	})
}

func TestInstrumentSessionDatastore(t *testing.T) {
	app := testApp(t)
	txn := app.StartTransaction("dynamodb-txn", nil, nil)

	ses := newSession()
	ses = InstrumentSession(ses)
	client := dynamodb.New(ses)

	input := &dynamodb.DescribeTableInput{
		TableName: aws.String("thebesttable"),
	}

	req, _ := client.DescribeTableRequest(input)
	req.HTTPRequest = newrelic.RequestWithTransactionContext(req.HTTPRequest, txn)

	err := req.Send()
	if nil != err {
		t.Error(err)
	}

	txn.End()

	app.(internal.Expect).ExpectMetrics(t, []internal.WantMetric{
		{"Datastore/DynamoDB/all", "", true, nil},
		{"Datastore/DynamoDB/allOther", "", true, nil},
		{"Datastore/all", "", true, nil},
		{"Datastore/allOther", "", true, nil},
		{"Datastore/instance/DynamoDB/dynamodb.us-west-2.amazonaws.com/unknown", "", false, nil},
		{"Datastore/operation/DynamoDB/DescribeTable", "", false, nil},
		{"Datastore/statement/DynamoDB/thebesttable/DescribeTable", "", false, nil},
		{"Datastore/statement/DynamoDB/thebesttable/DescribeTable", "OtherTransaction/Go/dynamodb-txn", false, nil},
		{"OtherTransaction/Go/dynamodb-txn", "", true, nil},
		{"OtherTransaction/all", "", true, nil},
	})
}

func TestInstrumentSessionExternalNoTxn(t *testing.T) {
	ses := newSession()
	ses = InstrumentSession(ses)
	client := lambda.New(ses)

	input := &lambda.InvokeInput{
		ClientContext:  aws.String("MyApp"),
		FunctionName:   aws.String("non-existent-function"),
		InvocationType: aws.String("Event"),
		LogType:        aws.String("Tail"),
		Payload:        []byte("{}"),
	}

	req, out := client.InvokeRequest(input)
	req.HTTPRequest = newrelic.RequestWithTransactionContext(req.HTTPRequest, nil)

	err := req.Send()
	if nil != err {
		t.Error(err)
	}
	if 200 != *out.StatusCode {
		t.Error("wrong status code on response", out.StatusCode)
	}
}

func TestInstrumentSessionDatastoreNoTxn(t *testing.T) {
	ses := newSession()
	ses = InstrumentSession(ses)
	client := dynamodb.New(ses)

	input := &dynamodb.DescribeTableInput{
		TableName: aws.String("thebesttable"),
	}

	req, _ := client.DescribeTableRequest(input)
	req.HTTPRequest = newrelic.RequestWithTransactionContext(req.HTTPRequest, nil)

	err := req.Send()
	if nil != err {
		t.Error(err)
	}
}

func TestInstrumentSessionExternalTxnNotInCtx(t *testing.T) {
	app := testApp(t)
	txn := app.StartTransaction("lambda-txn", nil, nil)

	ses := newSession()
	ses = InstrumentSession(ses)
	client := lambda.New(ses)

	input := &lambda.InvokeInput{
		ClientContext:  aws.String("MyApp"),
		FunctionName:   aws.String("non-existent-function"),
		InvocationType: aws.String("Event"),
		LogType:        aws.String("Tail"),
		Payload:        []byte("{}"),
	}

	req, out := client.InvokeRequest(input)

	err := req.Send()
	if nil != err {
		t.Error(err)
	}
	if 200 != *out.StatusCode {
		t.Error("wrong status code on response", out.StatusCode)
	}

	txn.End()

	app.(internal.Expect).ExpectMetrics(t, []internal.WantMetric{
		{"OtherTransaction/Go/lambda-txn", "", true, nil},
		{"OtherTransaction/all", "", true, nil},
	})
}

func TestInstrumentSessionDatastoreTxnNotInCtx(t *testing.T) {
	app := testApp(t)
	txn := app.StartTransaction("dynamodb-txn", nil, nil)

	ses := newSession()
	ses = InstrumentSession(ses)
	client := dynamodb.New(ses)

	input := &dynamodb.DescribeTableInput{
		TableName: aws.String("thebesttable"),
	}

	req, _ := client.DescribeTableRequest(input)

	err := req.Send()
	if nil != err {
		t.Error(err)
	}

	txn.End()

	app.(internal.Expect).ExpectMetrics(t, []internal.WantMetric{
		{"OtherTransaction/Go/dynamodb-txn", "", true, nil},
		{"OtherTransaction/all", "", true, nil},
	})
}

func TestDoublyInstrumented(t *testing.T) {
	countHandlers := func(hs *request.Handlers, t *testing.T, expected int) {
		if found := hs.Validate.Len(); expected != found {
			t.Error("unexpected number of Validate handlers found:", found)
		}
		if found := hs.Complete.Len(); expected != found {
			t.Error("unexpected number of Complete handlers found:", found)
		}
	}

	hs := &request.Handlers{}
	countHandlers(hs, t, 0)

	InstrumentHandlers(hs)
	countHandlers(hs, t, 1)

	InstrumentHandlers(hs)
	countHandlers(hs, t, 1)
}
