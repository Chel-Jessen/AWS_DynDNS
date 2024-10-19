package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	"io"
	"log"
	"net/http"
)

func getExternalIP() (string, error) {
	type IP struct {
		Ip string `json:"origin"`
	}

	resp, err := http.Get("http://httpbin.org/ip")
	if err != nil {
		return "", fmt.Errorf("could not get external IP: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("could not read response body: %v", err)
	}

	var ip IP
	if err := json.Unmarshal(body, &ip); err != nil {
		return "", fmt.Errorf("could not unmarshal JSON: %v", err)
	}

	return ip.Ip, nil
}

func updateRecord(accessKey, secretKey, hostedZoneID, domainName, subdomain, region string) error {
	ip, err := getExternalIP()
	if err != nil {
		return err
	}

	log.Printf("Current IP: %s\n", ip)

	creds := credentials.NewStaticCredentials(accessKey, secretKey, "")
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(region),
		Credentials: creds,
	})
	if err != nil {
		return err
	}

	svc := route53.New(sess)

	// Find the existing record
	recordName := subdomain + "." + domainName + "."
	input := &route53.ListResourceRecordSetsInput{
		HostedZoneId:    aws.String(hostedZoneID),
		StartRecordName: aws.String(recordName),
		StartRecordType: aws.String(route53.RRTypeA),
		MaxItems:        aws.String("1"),
	}

	output, err := svc.ListResourceRecordSets(input)
	if err != nil {
		return err
	}

	if len(output.ResourceRecordSets) == 0 || *output.ResourceRecordSets[0].Name != recordName {
		return fmt.Errorf("could not find A record for subdomain %s\n", subdomain)
	}

	var oldIP string
	for _, rr := range output.ResourceRecordSets {
		if *rr.Name == recordName && *rr.Type == "A" && len(rr.ResourceRecords) > 0 {
			oldIP = *rr.ResourceRecords[0].Value
			break
		}
	}

	if oldIP == ip {
		log.Printf("IP is up to date\n")
		return nil
	}

	// Update the record
	record := output.ResourceRecordSets[0]
	record.TTL = aws.Int64(300)
	record.ResourceRecords = []*route53.ResourceRecord{
		{
			Value: aws.String(ip),
		},
	}

	changeInput := &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(hostedZoneID),
		ChangeBatch: &route53.ChangeBatch{
			Changes: []*route53.Change{
				{
					Action:            aws.String(route53.ChangeActionUpsert),
					ResourceRecordSet: record,
				},
			},
			Comment: aws.String("Update record"),
		},
	}

	_, err = svc.ChangeResourceRecordSets(changeInput)
	if err != nil {
		return err
	}

	return nil
}

func main() {
	var accessKey string
	var secretKey string
	var hostedZoneID string
	var domainName string
	var subdomain string
	var region string

	flag.StringVar(&accessKey, "accessKey", "", "AWS Access Key")
	flag.StringVar(&secretKey, "secretKey", "", "AWS Secret Key")
	flag.StringVar(&hostedZoneID, "hostedZoneID", "", "Hosted Zone ID")
	flag.StringVar(&domainName, "domain", "", "Domain Name")
	flag.StringVar(&subdomain, "subdomain", "", "Subdomain")
	flag.StringVar(&region, "region", "us-east-1", "Region")
	flag.Parse()

	if accessKey == "" || secretKey == "" || hostedZoneID == "" || domainName == "" || subdomain == "" {
		log.Println("Missing Arguments")
		return
	}

	err := updateRecord(accessKey, secretKey, hostedZoneID, domainName, subdomain, region)
	if err != nil {
		log.Println(err)
	}
}
