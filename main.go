package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/quicksight"
	"github.com/aws/aws-sdk-go/service/sts"
)

// MyEvent is a sample event
type MyEvent struct {
	Name string `json:"name"`
}

type response struct {
	UTC time.Time `json:"utc"`
}

// HandleRequest handles incoming lambda request
func HandleRequest(ctx context.Context, event events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

	// setup CORS
	headers := make(map[string]string)
	headers["Access-Control-Allow-Methods"] = "GET,OPTIONS"
	headers["Content-Type"] = "application/json"

	headers["Access-Control-Allow-Origin"] = "*"

	body, err := GetDashboardURL("someone@mydomain.com", "11f05d8a-4a94-46c6-ad50-e6cb819934c5")
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}

	return events.APIGatewayProxyResponse{Body: body, StatusCode: 200, Headers: headers}, nil
}

func main() {
	lambda.Start(HandleRequest)
	// GetDashboardURL("someone@mydomain.com", "81d2ae9f-57bf-42b1-ad9e-9703718f36f6")
}

// GetDashboardURL handles incoming lambda request
func GetDashboardURL(userEmail string, dashboardID string) (string, error) {
	accountID, _ := GetAccountID()

	sess := session.Must(session.NewSession())
	roleName := "qer"

	awsAccountID := accountID
	iamRoleARN := "arn:aws:iam::" + awsAccountID + ":role/" + roleName
	// userEmail := "someone@mydomain.com"
	identityType := "IAM"
	userRegistrationRegion := GetAWSRegion()
	namespace := "default"

	// docs at https://docs.aws.amazon.com/quicksight/latest/APIReference/API_RegisterUser.html#QS-RegisterUser-request-UserRole
	userRole := "READER" // Valid Values: ADMIN | AUTHOR | READER | RESTRICTED_AUTHOR | RESTRICTED_READER

	// Step 1 - AssumeRole
	creds := stscreds.NewCredentials(sess, iamRoleARN)

	//Step 2: Register User. This might fail if user already exist, but no harm or foul if this fails due to UserAlready exists. Just continue
	client := quicksight.New(sess, &aws.Config{Credentials: creds, Region: &userRegistrationRegion})

	ruInput := quicksight.RegisterUserInput{
		AwsAccountId: &awsAccountID,
		Email:        &userEmail,
		IamArn:       &iamRoleARN,
		Namespace:    &namespace,
		IdentityType: &identityType,
		SessionName:  &userEmail,
		UserRole:     &userRole,
	}

	ruOutput, ruOutputError := client.RegisterUser(&ruInput)
	if ruOutputError != nil {
		fmt.Println(ruOutputError.Error())
	} else {
		fmt.Println(ruOutput.String())
	}

	// Step 3: Get the embeddedURL
	userDashboardRegion := GetAWSRegion()

	// Need to create separate client since dashboard region could be different from us-east-1 which is the user region
	client2 := quicksight.New(sess, &aws.Config{Credentials: creds, Region: &userDashboardRegion})
	userARN := "arn:aws:quicksight:us-east-1:" + awsAccountID + ":user/" + namespace + "/" + roleName + "/" + userEmail

	// options are /start, /start/analyses, /start/dashboards , /start/favorites , /dashboards/DashboardId, /analyses/AnalysisId
	// Docs at https://docs.aws.amazon.com/quicksight/latest/APIReference/API_GetSessionEmbedUrl.html
	entryPoint := "/dashboards/" + dashboardID

	eURLInput := quicksight.GetSessionEmbedUrlInput{
		AwsAccountId: &awsAccountID,
		EntryPoint:   &entryPoint,
		UserArn:      &userARN,
	}

	eURLOutput, errEmbed := client2.GetSessionEmbedUrl(&eURLInput)

	if errEmbed != nil {
		fmt.Println("\nStep 3.2 - ", errEmbed.Error())
		return errEmbed.Error(), errEmbed
	}

	return eURLOutput.String(), nil
}

// GetAccountID returns the current AWS account ID
func GetAccountID() (string, error) {
	svc := sts.New(session.New())
	input := &sts.GetCallerIdentityInput{}

	result, err := svc.GetCallerIdentity(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			fmt.Println(err.Error())
		}
		return err.Error(), err
	}

	fmt.Println(result)

	return *result.Account, nil
}

// GetAWSRegion gets AWS accountID that can be used elsewhere in application
func GetAWSRegion() string {
	region := os.Getenv("AWS_REGION") // Lambda provides this env variable

	if region == "" {
		return "us-west-2"
	}

	return region
}
