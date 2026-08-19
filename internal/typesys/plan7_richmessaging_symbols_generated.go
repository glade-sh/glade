package typesys

import "github.com/glade-sh/glade/internal/apexast"

// Code generated from public Salesforce product namespace declarations. DO NOT EDIT.

var plan7RichMessagingSymbolSpecs = []StandardSymbolSpec{
	{
		Name: "RichMessaging.CurrencyAmount",
		Properties: []StandardPropertySpec{
			{Name: "currency", Type: "String"},
		},
	},
	{
		Name: "RichMessaging.OrderBeneficiary",
		Properties: []StandardPropertySpec{
			{Name: "legalName", Type: "String"},
			{Name: "legalNameValue", Type: "String"},
			{Name: "taxIdentifierType", Type: "String"},
			{Name: "taxIdentifierTypeValue", Type: "String"},
			{Name: "taxIdentifierValue", Type: "String"},
			{Name: "taxIdentifierValueValue", Type: "String"},
		},
	},
	{
		Name: "RichMessaging.OrderContext",
		Properties: []StandardPropertySpec{
			{Name: "orderBeneficiary", Type: "RichMessaging.OrderBeneficiary"},
			{Name: "orderBeneficiaryValue", Type: "RichMessaging.OrderBeneficiary"},
			{Name: "orderExpiration", Type: "RichMessaging.OrderExpiration"},
			{Name: "orderExpirationValue", Type: "RichMessaging.OrderExpiration"},
			{Name: "orderType", Type: "String"},
			{Name: "orderTypeValue", Type: "RichMessaging.OrderType"},
			{Name: "paymentCheckoutBehavior", Type: "String"},
			{Name: "paymentCheckoutBehaviorValue", Type: "RichMessaging.PaymentCheckoutBehavior"},
			{Name: "paymentInitiation", Type: "RichMessaging.PaymentInitiation"},
			{Name: "paymentInitiationValue", Type: "RichMessaging.PaymentInitiation"},
			{Name: "paymentMethodHints", Type: "List<String>"},
			{Name: "paymentMethodHintsValue", Type: "List<RichMessaging.PaymentMethodHint>"},
		},
	},
	{
		Name: "RichMessaging.OrderExpiration",
		Properties: []StandardPropertySpec{
			{Name: "durationFromCreation", Type: "Long"},
			{Name: "durationFromCreationValue", Type: "Long"},
			{Name: "expirationDateTime", Type: "Datetime"},
			{Name: "expirationDateTimeValue", Type: "Datetime"},
		},
	},
	{
		Name: "RichMessaging.PaymentError",
		Properties: []StandardPropertySpec{
			{Name: "code", Type: "String"},
			{Name: "message", Type: "String"},
		},
	},
	{
		Name: "RichMessaging.PaymentGatewayProperties",
		Properties: []StandardPropertySpec{
			{Name: "callbackUrl", Type: "String"},
			{Name: "callbackUrlValue", Type: "String"},
			{Name: "returnUrl", Type: "String"},
			{Name: "returnUrlValue", Type: "String"},
		},
	},
	{
		Name: "RichMessaging.PaymentInitiation",
		Properties: []StandardPropertySpec{
			{Name: "initiationMode", Type: "String"},
			{Name: "initiationModeValue", Type: "RichMessaging.PaymentInitiationMode"},
		},
	},
	{
		Name: "RichMessaging.PaymentInitiationBoleto",
		Properties: []StandardPropertySpec{
			{Name: "documentNumber", Type: "String"},
			{Name: "documentNumberValue", Type: "String"},
		},
	},
	{
		Name: "RichMessaging.PaymentInitiationGateway",
		Properties: []StandardPropertySpec{
			{Name: "amount", Type: "Double"},
			{Name: "amountValue", Type: "Double"},
			{Name: "currency", Type: "String"},
			{Name: "currencyValue", Type: "String"},
			{Name: "gatewayProperties", Type: "RichMessaging.PaymentGatewayProperties"},
			{Name: "gatewayPropertiesValue", Type: "RichMessaging.PaymentGatewayProperties"},
			{Name: "metadata", Type: "Map<String,String>"},
			{Name: "metadataValue", Type: "Map<String,String>"},
			{Name: "orderId", Type: "String"},
			{Name: "orderIdValue", Type: "String"},
			{Name: "signature", Type: "String"},
			{Name: "signatureValue", Type: "String"},
		},
	},
	{
		Name: "RichMessaging.PaymentInitiationPix",
		Properties: []StandardPropertySpec{
			{Name: "endToEndId", Type: "String"},
			{Name: "endToEndIdValue", Type: "String"},
		},
	},
	{
		Name: "RichMessaging.PaymentInitiationPixKeyType",
		Kind: apexast.DeclarationEnum,
	},
	{
		Name: "RichMessaging.PaymentTransaction",
		Properties: []StandardPropertySpec{
			{Name: "amount", Type: "RichMessaging.CurrencyAmount"},
			{Name: "gatewayReference", Type: "String"},
			{Name: "refunds", Type: "List<RichMessaging.Refund>"},
			{Name: "status", Type: "RichMessaging.PaymentTransactionStatus"},
			{Name: "timestamp", Type: "Datetime"},
			{Name: "transactionId", Type: "String"},
			{Name: "transactionType", Type: "RichMessaging.PaymentTransactionType"},
		},
	},
	{
		Name: "RichMessaging.PaymentTransactionStatus",
		Kind: apexast.DeclarationEnum,
	},
	{
		Name: "RichMessaging.PaymentTransactionType",
		Kind: apexast.DeclarationEnum,
	},
	{
		Name: "RichMessaging.ProcessPaymentStatusHandler",
		Kind: apexast.DeclarationInterface,
		Methods: []StandardMethodSpec{
			{Name: "processPaymentStatus", ReturnType: "RichMessaging.ProcessPaymentStatusResult", Parameters: []string{"RichMessaging.ProcessPaymentStatusRequest"}},
		},
	},
	{
		Name: "RichMessaging.ProcessPaymentStatusRequest",
		Properties: []StandardPropertySpec{
			{Name: "amount", Type: "RichMessaging.CurrencyAmount"},
			{Name: "error", Type: "RichMessaging.PaymentError"},
			{Name: "orderId", Type: "String"},
			{Name: "paymentMethod", Type: "RichMessaging.PaymentMethod"},
			{Name: "paymentTransaction", Type: "RichMessaging.PaymentTransaction"},
			{Name: "transactionId", Type: "String"},
			{Name: "transactionStatus", Type: "RichMessaging.PaymentTransactionStatus"},
			{Name: "transactionType", Type: "RichMessaging.PaymentTransactionType"},
		},
	},
	{
		Name: "RichMessaging.ProcessPaymentStatusResult",
		Properties: []StandardPropertySpec{
			{Name: "message", Type: "String"},
			{Name: "status", Type: "RichMessaging.ProcessPaymentResultStatus"},
		},
	},
	{
		Name: "RichMessaging.Refund",
		Properties: []StandardPropertySpec{
			{Name: "amount", Type: "RichMessaging.CurrencyAmount"},
			{Name: "gatewayReference", Type: "String"},
			{Name: "reason", Type: "String"},
			{Name: "refundId", Type: "String"},
			{Name: "status", Type: "RichMessaging.PaymentTransactionStatus"},
			{Name: "timestamp", Type: "Datetime"},
		},
	},
}
