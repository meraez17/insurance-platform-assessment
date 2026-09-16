package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"assessment/orchestrator/internal/domain"
	"assessment/orchestrator/internal/storage"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Processor struct { repo *storage.Repository; channel *amqp.Channel; client *http.Client; simulatorURL,issuerURL string }

func NewProcessor(repo *storage.Repository,ch *amqp.Channel,simulatorURL,issuerURL string)*Processor{return &Processor{repo:repo,channel:ch,client:&http.Client{Timeout:3*time.Second},simulatorURL:simulatorURL,issuerURL:issuerURL}}

func (p *Processor) Run(ctx context.Context) error {
	queue,err:=p.channel.QueueDeclare("insurance.processor",true,false,false,false,amqp.Table{"x-dead-letter-exchange":"insurance.dlx"});if err!=nil{return err}
	if err=p.channel.QueueBind(queue.Name,"#","insurance.events",false,nil);err!=nil{return err}
	deliveries,err:=p.channel.Consume(queue.Name,"",false,false,false,false,nil);if err!=nil{return err}
	for { select { case<-ctx.Done():return ctx.Err();case message,ok:=<-deliveries:if !ok{return errors.New("processor delivery channel closed")};if err=p.process(ctx,message);err!=nil{_ = message.Nack(false,true);time.Sleep(500*time.Millisecond)}else{_ = message.Ack(false)} } }
}

func(p *Processor)process(ctx context.Context,message amqp.Delivery)error{
	var payload map[string]any;if err:=json.Unmarshal(message.Body,&payload);err!=nil{return err}
	orderID,_:=payload["orderId"].(string);if orderID==""{return errors.New("event has no orderId")}
	switch message.RoutingKey {
	case "ORDER_CREATED": return p.quoteAndPay(ctx,orderID)
	case "PAYMENT_APPROVED","ISSUANCE_RETRY_REQUESTED": return p.issue(ctx,orderID)
	default:return nil
	}
}

func(p *Processor)quoteAndPay(ctx context.Context,orderID string)error{
	order,err:=p.repo.Get(ctx,orderID);if err!=nil{return err}
	var quote struct{PremiumCents int64 `json:"premiumCents"`}
	if err=p.post(ctx,p.simulatorURL+"/quote",map[string]any{"plate":order.MaskedPlate},&quote);err!=nil{return err}
	order,err=p.repo.ApplyQuote(ctx,orderID,quote.PremiumCents);if err!=nil{return err}
	if err=p.post(ctx,p.simulatorURL+"/payments",map[string]any{"orderId":order.ID,"amountCents":order.PremiumCents},nil);err!=nil{return err}
	_,err=p.repo.Transition(ctx,order.ID,domain.Quoted,domain.PaymentPending,"PAYMENT_INITIATED",map[string]any{"orderId":order.ID});return err
}

func(p *Processor)issue(ctx context.Context,orderID string)error{
	order,err:=p.repo.Get(ctx,orderID);if err!=nil{return err}
	if order.Status==domain.Paid { order,err=p.repo.Transition(ctx,order.ID,domain.Paid,domain.Issuing,"POLICY_ISSUANCE_REQUESTED",map[string]any{"orderId":order.ID});if err!=nil{return err} }
	if order.Status!=domain.Issuing{return fmt.Errorf("order %s is not ready for issuance",order.ID)}
	var issued struct{PolicyNumber string `json:"policyNumber"`}
	err=p.post(ctx,p.issuerURL+"/policies",map[string]any{"externalRef":order.ExternalRef,"vehiclePlate":order.MaskedPlate,"premiumCents":order.PremiumCents},&issued)
	if err==nil{return p.markIssued(ctx,order.ID,issued.PolicyNumber)}
	request,_:=http.NewRequestWithContext(ctx,http.MethodGet,p.issuerURL+"/policies/by-reference/"+order.ExternalRef,nil)
	response,lookupErr:=p.client.Do(request);if lookupErr!=nil{return err};defer response.Body.Close()
	if response.StatusCode==http.StatusNotFound{return err};if response.StatusCode!=http.StatusOK{return fmt.Errorf("reconciliation failed: %s",response.Status)}
	if decodeErr:=json.NewDecoder(response.Body).Decode(&issued);decodeErr!=nil{return decodeErr}
	return p.markIssued(ctx,order.ID,issued.PolicyNumber)
}

func(p *Processor)markIssued(ctx context.Context,orderID,policy string)error{if policy==""{return errors.New("issuer returned empty policy number")};_,err:=p.repo.MarkIssued(ctx,orderID,policy);return err}
func(p *Processor)post(ctx context.Context,url string,payload any,target any)error{body,_:=json.Marshal(payload);request,_:=http.NewRequestWithContext(ctx,http.MethodPost,url,bytes.NewReader(body));request.Header.Set("Content-Type","application/json");response,err:=p.client.Do(request);if err!=nil{return err};defer response.Body.Close();if response.StatusCode<200||response.StatusCode>=300{return fmt.Errorf("dependency returned %s",response.Status)};if target!=nil{return json.NewDecoder(response.Body).Decode(target)};return nil}
