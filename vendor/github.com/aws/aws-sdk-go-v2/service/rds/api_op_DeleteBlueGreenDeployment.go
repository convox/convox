// Code generated by smithy-go-codegen DO NOT EDIT.

package rds

import (
	"context"
	"fmt"
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// Deletes a blue/green deployment.
//
// For more information, see [Using Amazon RDS Blue/Green Deployments for database updates] in the Amazon RDS User Guide and [Using Amazon RDS Blue/Green Deployments for database updates] in the Amazon
// Aurora User Guide.
//
// [Using Amazon RDS Blue/Green Deployments for database updates]: https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/blue-green-deployments.html
func (c *Client) DeleteBlueGreenDeployment(ctx context.Context, params *DeleteBlueGreenDeploymentInput, optFns ...func(*Options)) (*DeleteBlueGreenDeploymentOutput, error) {
	if params == nil {
		params = &DeleteBlueGreenDeploymentInput{}
	}

	result, metadata, err := c.invokeOperation(ctx, "DeleteBlueGreenDeployment", params, optFns, c.addOperationDeleteBlueGreenDeploymentMiddlewares)
	if err != nil {
		return nil, err
	}

	out := result.(*DeleteBlueGreenDeploymentOutput)
	out.ResultMetadata = metadata
	return out, nil
}

type DeleteBlueGreenDeploymentInput struct {

	// The unique identifier of the blue/green deployment to delete. This parameter
	// isn't case-sensitive.
	//
	// Constraints:
	//
	//   - Must match an existing blue/green deployment identifier.
	//
	// This member is required.
	BlueGreenDeploymentIdentifier *string

	// Specifies whether to delete the resources in the green environment. You can't
	// specify this option if the blue/green deployment [status]is SWITCHOVER_COMPLETED .
	//
	// [status]: https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_BlueGreenDeployment.html
	DeleteTarget *bool

	noSmithyDocumentSerde
}

type DeleteBlueGreenDeploymentOutput struct {

	// Details about a blue/green deployment.
	//
	// For more information, see [Using Amazon RDS Blue/Green Deployments for database updates] in the Amazon RDS User Guide and [Using Amazon RDS Blue/Green Deployments for database updates] in the Amazon
	// Aurora User Guide.
	//
	// [Using Amazon RDS Blue/Green Deployments for database updates]: https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/blue-green-deployments.html
	BlueGreenDeployment *types.BlueGreenDeployment

	// Metadata pertaining to the operation's result.
	ResultMetadata middleware.Metadata

	noSmithyDocumentSerde
}

func (c *Client) addOperationDeleteBlueGreenDeploymentMiddlewares(stack *middleware.Stack, options Options) (err error) {
	if err := stack.Serialize.Add(&setOperationInputMiddleware{}, middleware.After); err != nil {
		return err
	}
	err = stack.Serialize.Add(&awsAwsquery_serializeOpDeleteBlueGreenDeployment{}, middleware.After)
	if err != nil {
		return err
	}
	err = stack.Deserialize.Add(&awsAwsquery_deserializeOpDeleteBlueGreenDeployment{}, middleware.After)
	if err != nil {
		return err
	}
	if err := addProtocolFinalizerMiddlewares(stack, options, "DeleteBlueGreenDeployment"); err != nil {
		return fmt.Errorf("add protocol finalizers: %v", err)
	}

	if err = addlegacyEndpointContextSetter(stack, options); err != nil {
		return err
	}
	if err = addSetLoggerMiddleware(stack, options); err != nil {
		return err
	}
	if err = addClientRequestID(stack); err != nil {
		return err
	}
	if err = addComputeContentLength(stack); err != nil {
		return err
	}
	if err = addResolveEndpointMiddleware(stack, options); err != nil {
		return err
	}
	if err = addComputePayloadSHA256(stack); err != nil {
		return err
	}
	if err = addRetry(stack, options); err != nil {
		return err
	}
	if err = addRawResponseToMetadata(stack); err != nil {
		return err
	}
	if err = addRecordResponseTiming(stack); err != nil {
		return err
	}
	if err = addClientUserAgent(stack, options); err != nil {
		return err
	}
	if err = smithyhttp.AddErrorCloseResponseBodyMiddleware(stack); err != nil {
		return err
	}
	if err = smithyhttp.AddCloseResponseBodyMiddleware(stack); err != nil {
		return err
	}
	if err = addSetLegacyContextSigningOptionsMiddleware(stack); err != nil {
		return err
	}
	if err = addOpDeleteBlueGreenDeploymentValidationMiddleware(stack); err != nil {
		return err
	}
	if err = stack.Initialize.Add(newServiceMetadataMiddleware_opDeleteBlueGreenDeployment(options.Region), middleware.Before); err != nil {
		return err
	}
	if err = addRecursionDetection(stack); err != nil {
		return err
	}
	if err = addRequestIDRetrieverMiddleware(stack); err != nil {
		return err
	}
	if err = addResponseErrorMiddleware(stack); err != nil {
		return err
	}
	if err = addRequestResponseLogging(stack, options); err != nil {
		return err
	}
	if err = addDisableHTTPSMiddleware(stack, options); err != nil {
		return err
	}
	return nil
}

func newServiceMetadataMiddleware_opDeleteBlueGreenDeployment(region string) *awsmiddleware.RegisterServiceMetadata {
	return &awsmiddleware.RegisterServiceMetadata{
		Region:        region,
		ServiceID:     ServiceID,
		OperationName: "DeleteBlueGreenDeployment",
	}
}