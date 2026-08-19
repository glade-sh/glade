package typesys

import "github.com/glade-sh/glade/internal/apexast"

// Code generated from public Salesforce product namespace declarations. DO NOT EDIT.

var plan7RichMessagingSymbolSpecs = []StandardSymbolSpec{
	{
		Name: "RichMessaging.AbstractTiming",
	},
	{
		Name: "RichMessaging.Address",
		Properties: []StandardPropertySpec{
			{Name: "addressLines", Type: "Object"},
			{Name: "addressLinesValue", Type: "Object"},
			{Name: "administrativeArea", Type: "Object"},
			{Name: "administrativeAreaValue", Type: "Object"},
			{Name: "country", Type: "Object"},
			{Name: "countryCode", Type: "Object"},
			{Name: "countryCodeValue", Type: "Object"},
			{Name: "countryValue", Type: "Object"},
			{Name: "locality", Type: "Object"},
			{Name: "localityValue", Type: "Object"},
			{Name: "postalCode", Type: "Object"},
			{Name: "postalCodeValue", Type: "Object"},
			{Name: "subAdministrativeArea", Type: "Object"},
			{Name: "subAdministrativeAreaValue", Type: "Object"},
			{Name: "subLocality", Type: "Object"},
			{Name: "subLocalityValue", Type: "Object"},
		},
	},
	{
		Name:         "RichMessaging.AddressableContact",
		Constructors: [][]string{{"Object", "Object", "Object", "Object", "Object", "Object", "Object"}},
		Properties: []StandardPropertySpec{
			{Name: "emailAddress", Type: "Object"},
			{Name: "familyName", Type: "Object"},
			{Name: "givenName", Type: "Object"},
			{Name: "phoneNumber", Type: "Object"},
			{Name: "phoneticFamilyName", Type: "Object"},
			{Name: "phoneticGivenName", Type: "Object"},
			{Name: "postalAddress", Type: "Object"},
		},
	},
	{
		Name: "RichMessaging.AuthRequestHandler",
		Kind: apexast.DeclarationInterface,
		Methods: []StandardMethodSpec{
			{Name: "handleAuthRequest", ReturnType: "RichMessaging.AuthRequestResult", Parameters: []string{"RichMessaging.AuthRequestResponse"}},
		},
	},
	{
		Name:         "RichMessaging.AuthRequestResponse",
		Constructors: [][]string{{"String", "String", "String"}},
		Methods: []StandardMethodSpec{
			{Name: "getAccessToken", ReturnType: "String"},
			{Name: "getAuthProviderName", ReturnType: "String"},
			{Name: "getContextRecordId", ReturnType: "String"},
		},
	},
	{
		Name:         "RichMessaging.AuthRequestResult",
		Constructors: [][]string{{"System.PageReference", "RichMessaging.AuthRequestResultStatus", "Datetime"}},
		Properties: []StandardPropertySpec{
			{Name: "expirationDateTime", Type: "Object"},
			{Name: "redirectPageReference", Type: "Object"},
			{Name: "resultStatus", Type: "Object"},
		},
	},
	{
		Name: "RichMessaging.AuthRequestResultStatus",
		Kind: apexast.DeclarationEnum,
	},
	{
		Name: "RichMessaging.CurrencyAmount",
		Properties: []StandardPropertySpec{
			{Name: "amount", Type: "Object"},
			{Name: "currency", Type: "Object"},
		},
	},
	{
		Name:         "RichMessaging.DeferredTiming",
		Constructors: [][]string{{}, {"Datetime"}},
		Properties: []StandardPropertySpec{
			{Name: "deferredDate", Type: "Object"},
			{Name: "deferredDateValue", Type: "Object"},
			{Name: "timingType", Type: "Object"},
		},
	},
	{
		Name: "RichMessaging.MessageDefinitionInputParameter",
		Properties: []StandardPropertySpec{
			{Name: "booleanValue", Type: "Object"},
			{Name: "booleanValues", Type: "Object"},
			{Name: "dateTimeValue", Type: "Object"},
			{Name: "dateTimeValues", Type: "Object"},
			{Name: "dateValue", Type: "Object"},
			{Name: "dateValues", Type: "Object"},
			{Name: "name", Type: "Object"},
			{Name: "numberValue", Type: "Object"},
			{Name: "numberValues", Type: "Object"},
			{Name: "recordIdValue", Type: "Object"},
			{Name: "recordIdValues", Type: "Object"},
			{Name: "textValue", Type: "Object"},
			{Name: "textValues", Type: "Object"},
		},
	},
	{
		Name: "RichMessaging.OrderBeneficiary",
		Properties: []StandardPropertySpec{
			{Name: "legalName", Type: "Object"},
			{Name: "legalNameValue", Type: "Object"},
			{Name: "taxIdentifierType", Type: "Object"},
			{Name: "taxIdentifierTypeValue", Type: "Object"},
			{Name: "taxIdentifierValue", Type: "Object"},
			{Name: "taxIdentifierValueValue", Type: "Object"},
		},
	},
	{
		Name: "RichMessaging.OrderContext",
		Properties: []StandardPropertySpec{
			{Name: "orderBeneficiary", Type: "Object"},
			{Name: "orderBeneficiaryValue", Type: "Object"},
			{Name: "orderExpiration", Type: "Object"},
			{Name: "orderExpirationValue", Type: "Object"},
			{Name: "orderType", Type: "Object"},
			{Name: "orderTypeValue", Type: "Object"},
			{Name: "paymentCheckoutBehavior", Type: "Object"},
			{Name: "paymentCheckoutBehaviorValue", Type: "Object"},
			{Name: "paymentInitiation", Type: "Object"},
			{Name: "paymentInitiationValue", Type: "Object"},
			{Name: "paymentMethodHints", Type: "Object"},
			{Name: "paymentMethodHintsValue", Type: "Object"},
		},
	},
	{
		Name: "RichMessaging.OrderExpiration",
		Properties: []StandardPropertySpec{
			{Name: "durationFromCreation", Type: "Object"},
			{Name: "durationFromCreationValue", Type: "Object"},
			{Name: "expirationDateTime", Type: "Object"},
			{Name: "expirationDateTimeValue", Type: "Object"},
		},
	},
	{
		Name: "RichMessaging.OrderItemCategory",
		Kind: apexast.DeclarationEnum,
	},
	{
		Name: "RichMessaging.OrderItemCommerceAttributes",
		Properties: []StandardPropertySpec{
			{Name: "category", Type: "Object"},
			{Name: "categoryValue", Type: "Object"},
			{Name: "importer", Type: "Object"},
			{Name: "importerValue", Type: "Object"},
			{Name: "retailerId", Type: "Object"},
			{Name: "retailerIdValue", Type: "Object"},
		},
	},
	{
		Name: "RichMessaging.OrderItemImporter",
		Properties: []StandardPropertySpec{
			{Name: "address", Type: "Object"},
			{Name: "addressValue", Type: "Object"},
			{Name: "countryOfOrigin", Type: "Object"},
			{Name: "countryOfOriginValue", Type: "Object"},
			{Name: "name", Type: "Object"},
			{Name: "nameValue", Type: "Object"},
		},
	},
	{
		Name: "RichMessaging.OrderType",
		Kind: apexast.DeclarationEnum,
	},
	{
		Name: "RichMessaging.PaymentCheckoutBehavior",
		Kind: apexast.DeclarationEnum,
	},
	{
		Name: "RichMessaging.PaymentError",
		Properties: []StandardPropertySpec{
			{Name: "code", Type: "Object"},
			{Name: "message", Type: "Object"},
		},
	},
	{
		Name: "RichMessaging.PaymentGatewayProperties",
		Properties: []StandardPropertySpec{
			{Name: "callbackUrl", Type: "Object"},
			{Name: "callbackUrlValue", Type: "Object"},
			{Name: "returnUrl", Type: "Object"},
			{Name: "returnUrlValue", Type: "Object"},
		},
	},
	{
		Name: "RichMessaging.PaymentInitiation",
		Properties: []StandardPropertySpec{
			{Name: "boleto", Type: "Object"},
			{Name: "boletoValue", Type: "Object"},
			{Name: "gateway", Type: "Object"},
			{Name: "gatewayValue", Type: "Object"},
			{Name: "initiationMode", Type: "Object"},
			{Name: "initiationModeValue", Type: "Object"},
			{Name: "pix", Type: "Object"},
			{Name: "pixValue", Type: "Object"},
		},
	},
	{
		Name: "RichMessaging.PaymentInitiationBoleto",
		Properties: []StandardPropertySpec{
			{Name: "documentNumber", Type: "Object"},
			{Name: "documentNumberValue", Type: "Object"},
		},
	},
	{
		Name: "RichMessaging.PaymentInitiationGateway",
		Properties: []StandardPropertySpec{
			{Name: "amount", Type: "Object"},
			{Name: "amountValue", Type: "Object"},
			{Name: "currency", Type: "Object"},
			{Name: "currencyValue", Type: "Object"},
			{Name: "gatewayProperties", Type: "Object"},
			{Name: "gatewayPropertiesValue", Type: "Object"},
			{Name: "metadata", Type: "Object"},
			{Name: "metadataValue", Type: "Object"},
			{Name: "orderId", Type: "Object"},
			{Name: "orderIdValue", Type: "Object"},
			{Name: "signature", Type: "Object"},
			{Name: "signatureValue", Type: "Object"},
		},
	},
	{
		Name: "RichMessaging.PaymentInitiationMode",
		Kind: apexast.DeclarationEnum,
	},
	{
		Name: "RichMessaging.PaymentInitiationPix",
		Properties: []StandardPropertySpec{
			{Name: "endToEndId", Type: "Object"},
			{Name: "endToEndIdValue", Type: "Object"},
			{Name: "key", Type: "Object"},
			{Name: "keyType", Type: "Object"},
			{Name: "keyTypeValue", Type: "Object"},
			{Name: "keyValue", Type: "Object"},
		},
	},
	{
		Name: "RichMessaging.PaymentInitiationPixKeyType",
		Kind: apexast.DeclarationEnum,
	},
	{
		Name: "RichMessaging.PaymentItemStatus",
		Kind: apexast.DeclarationEnum,
	},
	{
		Name:         "RichMessaging.PaymentLineItem",
		Constructors: [][]string{{}, {"String", "Double"}, {"String", "Double", "RichMessaging.AbstractTiming"}},
		Properties: []StandardPropertySpec{
			{Name: "amount", Type: "Object"},
			{Name: "amountValue", Type: "Object"},
			{Name: "automaticReloadPaymentThresholdAmount", Type: "Object"},
			{Name: "automaticReloadPaymentThresholdAmountValue", Type: "Object"},
			{Name: "commerce", Type: "Object"},
			{Name: "commerceValue", Type: "Object"},
			{Name: "discount", Type: "Object"},
			{Name: "discountValue", Type: "Object"},
			{Name: "label", Type: "Object"},
			{Name: "labelValue", Type: "Object"},
			{Name: "lineItemType", Type: "Object"},
			{Name: "quantity", Type: "Object"},
			{Name: "quantityValue", Type: "Object"},
			{Name: "saleAmount", Type: "Object"},
			{Name: "saleAmountValue", Type: "Object"},
			{Name: "status", Type: "Object"},
			{Name: "statusValue", Type: "Object"},
			{Name: "timing", Type: "Object"},
			{Name: "timingValue", Type: "Object"},
		},
	},
	{
		Name:         "RichMessaging.PaymentMethod",
		Constructors: [][]string{{"String", "String", "String"}},
		Properties: []StandardPropertySpec{
			{Name: "displayName", Type: "Object"},
			{Name: "network", Type: "Object"},
			{Name: "paymentType", Type: "Object"},
		},
	},
	{
		Name: "RichMessaging.PaymentMethodHint",
		Kind: apexast.DeclarationEnum,
	},
	{
		Name: "RichMessaging.PaymentTransaction",
		Properties: []StandardPropertySpec{
			{Name: "amount", Type: "Object"},
			{Name: "gatewayReference", Type: "Object"},
			{Name: "refunds", Type: "Object"},
			{Name: "status", Type: "Object"},
			{Name: "timestamp", Type: "Object"},
			{Name: "transactionId", Type: "Object"},
			{Name: "transactionType", Type: "Object"},
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
		Name:         "RichMessaging.PostalAddress",
		Constructors: [][]string{{"List<String>", "String", "String", "String", "String", "String", "String", "String"}},
		Properties: []StandardPropertySpec{
			{Name: "addressLines", Type: "Object"},
			{Name: "administrativeArea", Type: "Object"},
			{Name: "country", Type: "Object"},
			{Name: "countryCode", Type: "Object"},
			{Name: "locality", Type: "Object"},
			{Name: "postalCode", Type: "Object"},
			{Name: "subAdministrativeArea", Type: "Object"},
			{Name: "subLocality", Type: "Object"},
		},
	},
	{
		Name: "RichMessaging.ProcessFormHandler",
		Kind: apexast.DeclarationInterface,
		Properties: []StandardPropertySpec{
			{Name: "processFormRequest", Type: "Object"},
		},
	},
	{
		Name: "RichMessaging.ProcessPaymentHandler",
		Kind: apexast.DeclarationInterface,
		Methods: []StandardMethodSpec{
			{Name: "processPaymentRequest", ReturnType: "RichMessaging.ProcessPaymentResult", Parameters: []string{"RichMessaging.ProcessPaymentRequest"}},
		},
	},
	{
		Name:         "RichMessaging.ProcessPaymentRequest",
		Constructors: [][]string{{"String", "String", "RichMessaging.AddressableContact", "RichMessaging.AddressableContact", "RichMessaging.PaymentMethod", "RichMessaging.ShippingMethod", "String"}},
		Properties: []StandardPropertySpec{
			{Name: "billingContact", Type: "Object"},
			{Name: "contextRecordId", Type: "Object"},
			{Name: "paymentData", Type: "Object"},
			{Name: "paymentMethod", Type: "Object"},
			{Name: "shippingContact", Type: "Object"},
			{Name: "shippingMethod", Type: "Object"},
			{Name: "transactionIdentifier", Type: "Object"},
		},
	},
	{
		Name:         "RichMessaging.ProcessPaymentResult",
		Constructors: [][]string{{"RichMessaging.ProcessPaymentResultStatus"}, {"RichMessaging.ProcessPaymentResultStatus", "String"}},
		Properties: []StandardPropertySpec{
			{Name: "errorMessage", Type: "Object"},
			{Name: "resultStatus", Type: "Object"},
		},
	},
	{
		Name: "RichMessaging.ProcessPaymentResultStatus",
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
			{Name: "amount", Type: "Object"},
			{Name: "error", Type: "Object"},
			{Name: "orderId", Type: "Object"},
			{Name: "paymentMethod", Type: "Object"},
			{Name: "paymentTransaction", Type: "Object"},
			{Name: "transactionId", Type: "Object"},
			{Name: "transactionStatus", Type: "Object"},
			{Name: "transactionType", Type: "Object"},
		},
	},
	{
		Name: "RichMessaging.ProcessPaymentStatusResult",
		Properties: []StandardPropertySpec{
			{Name: "message", Type: "Object"},
			{Name: "status", Type: "Object"},
		},
	},
	{
		Name:         "RichMessaging.RecurringTiming",
		Constructors: [][]string{{}, {"Date", "Date", "Integer", "RichMessaging.TimingIntervalUnit"}},
		Properties: []StandardPropertySpec{
			{Name: "endDate", Type: "Object"},
			{Name: "endDateValue", Type: "Object"},
			{Name: "intervalCount", Type: "Object"},
			{Name: "intervalCountValue", Type: "Object"},
			{Name: "intervalUnit", Type: "Object"},
			{Name: "intervalUnitValue", Type: "Object"},
			{Name: "startDate", Type: "Object"},
			{Name: "startDateValue", Type: "Object"},
			{Name: "timingType", Type: "Object"},
		},
	},
	{
		Name: "RichMessaging.Refund",
		Properties: []StandardPropertySpec{
			{Name: "amount", Type: "Object"},
			{Name: "gatewayReference", Type: "Object"},
			{Name: "reason", Type: "Object"},
			{Name: "refundId", Type: "Object"},
			{Name: "status", Type: "Object"},
			{Name: "timestamp", Type: "Object"},
		},
	},
	{
		Name: "RichMessaging.RichMessaging",
	},
	{
		Name:         "RichMessaging.ShippingMethod",
		Constructors: [][]string{{}, {"String", "Double", "String", "String"}},
		Properties: []StandardPropertySpec{
			{Name: "amount", Type: "Object"},
			{Name: "amountValue", Type: "Object"},
			{Name: "detail", Type: "Object"},
			{Name: "detailValue", Type: "Object"},
			{Name: "identifier", Type: "Object"},
			{Name: "identifierValue", Type: "Object"},
			{Name: "label", Type: "Object"},
			{Name: "labelValue", Type: "Object"},
			{Name: "shippingMethodType", Type: "Object"},
		},
	},
	{
		Name:         "RichMessaging.TimeSlotOption",
		Constructors: [][]string{{}, {"Datetime", "Datetime"}, {"Datetime", "Integer"}},
		Properties: []StandardPropertySpec{
			{Name: "duration", Type: "Object"},
			{Name: "durationValue", Type: "Object"},
			{Name: "endTimeValue", Type: "Object"},
			{Name: "startTime", Type: "Object"},
			{Name: "startTimeValue", Type: "Object"},
		},
	},
	{
		Name: "RichMessaging.TimingIntervalUnit",
		Kind: apexast.DeclarationEnum,
	},
	{
		Name: "RichMessaging.TimingType",
		Kind: apexast.DeclarationEnum,
	},
}
