ZainCash Merchant Payment Gateway – Integration Guide
Thank you for choosing ZainCash, Iraq’s leading mobile payment network. The ZainCash Merchant Payment Gateway provides a secure, scalable, and seamless way for businesses to accept digital payments. By integrating our solution, you offer your customers a fast and familiar checkout experience directly through their mobile wallets.

Version

1.0

Updated

22 Jan, 2026

Overview

The ZainCash Payment Gateway v2 lets you accept payments from ZainCash wallets using a secure redirect flow, with real-time status updates via API and webhooks.

Base URL

https://pg-api-uat.zaincash.iq


Payment Flow

A typical payment using the Payment Gateway v2 looks like this:

Customer starts payment on your website or mobile app.
Your backend authenticates with ZainCash using POST /oauth2/token.
You create a transaction using POST /api/v2/payment-gateway/transaction/init.
You redirect the customer to the Payment Gateway redirectUrl.
Customer completes the payment (including OTP).
ZainCash redirects the customer back to your successUrl or failureUrl with a JWT token.
Your backend verifies the JWT and/or calls the Inquiry API to confirm final status.
Optionally, you receive webhook notifications for status changes and refunds.
For most merchants, the source of truth should be the webhook event. Use the redirect token for UX and the inquiry endpoint as a fallback.

Environments

Test: https://pg-api-uat.zaincash.iq
Production: Provided during onboarding.
Use separate client credentials per environment.

High-level capabilities

Wallet.
OTP-based authentication where applicable.
Inquiry and reversal APIs.
JWT-based redirect and webhook callbacks.
Quickstart

This is the fastest way to go from zero to a working payment:

Obtain your client_id, client_secret, and API key from ZainCash.
Get an OAuth2 access token using client_credentials grant.
Call the transaction/init endpoint to create a payment.
Redirect the user to redirectUrl from the response.
Handle the redirect to your successUrl/failureUrl using the token.
Verify the JWT and update your order status.
1. Get Access Token

POST

/oauth2/token


curl
JavaScript
C#
Java
PHP

curl --location 'https://pg-api-uat.zaincash.iq/oauth2/token' \
--header 'Content-Type: application/x-www-form-urlencoded' \
--data-urlencode 'grant_type=client_credentials' \
--data-urlencode 'client_id=145063190....' \
--data-urlencode 'client_secret=I0gd8hJ6Rn5....' \
--data-urlencode 'scope=payment:read payment:write reverse:write '
    
2. Create Payment

POST

/api/v2/payment-gateway/transaction/init


curl
JavaScript
C#
Java
PHP

curl --location 'https://pg-api-uat.zaincash.iq/api/v2/payment-gateway/transaction/init' \
--header 'Content-Type: application/json' \
--header 'Authorization: Bearer eyJraWQiOiJwYXltZW50LWdhdGV3YXkta2V5IiwiYWxnIjoiUlMyNTYifQ.eyJz.....' \
--data-raw '{
  "language": "en", // please make sure to choose the correct language based on your application locale.
  "externalReferenceId": "76554912-1810-4bf3-8b17-1bfc2c9fa6bf",
  "orderId": "moi-21323123vc",
  "serviceType": "Delivery",
  "amount": {
    "value": "3000",
    "currency": "IQD"
  },
  "customer": {
    "phone": "964XXXXXXXXXX"
  },
  "redirectUrls": {
    "successUrl": "https://merchant.com/payment/success",
    "failureUrl": "https://merchant.com/payment/failure"
  }
  }'
    
Use a unique externalReferenceId each logical payment attempt. This acts as an idempotency key and helps you reconcile payments on your side.

Authentication

All API requests (except /oauth2/token) require a valid bearer token in the Authorization header.

Endpoint

POST

/oauth2/token


Header

Value

Content-Type
application/x-www-form-urlencoded
Body Parameters

Field

Type

Required

Description

grant_type
string	yes	Must be client_credentials.
client_id
string	yes	Your client ID.
client_secret
string	yes	Your client secret.
scope
string	yes	Space-separated scopes, e.g., payment:read payment:write.
Get Access Token

curl
JavaScript
C#
Java
PHP

curl --location 'https://pg-api-uat.zaincash.iq/oauth2/token' \
--header 'Content-Type: application/x-www-form-urlencoded' \
--data-urlencode 'grant_type=client_credentials' \
--data-urlencode 'client_id=145063190....' \
--data-urlencode 'client_secret=I0gd8hJ6Rn5....' \
--data-urlencode 'scope=payment:read payment:write reverse:write '
    
Response

Json
{
    "access_token": "access_token",
    "scope": "reverse:write payment:write payment:read ",
    "token_type": "Bearer",
    "expires_in": 86399
}
Use the access token in all subsequent API requests.

Json

Authorization: Bearer <access_token>
Create Payment

Create a new payment session and obtain the redirectUrl where you should send the customer to complete the payment.

POST

/api/v2/payment-gateway/transaction/init


Scopes

payment:read

Header

Value

Authorization
Bearer <access_token>
Content-Type
application/json
Field

Type

Required

Description

language
string	yes	Please make sure to choose the correct language based on your application locale. Supported values: "En" for English, "Ar" for Arabic And "Ku" for Kurdish.
externalReferenceId
string (UUID)	yes	Unique per request; use for idempotency and reconciliation.
orderId
string	yes	Your internal order identifier.
serviceType
string	yes	Service identifier (e.g., JAWS) provided by ZainCash.
amount.value
string / number	yes	Transaction amount.
amount.currency
string	yes	Must be IQD.
customer.phone
string	optional	Customer phone in international format (e.g., 96477...).
redirectUrls.successUrl
string	yes	Where the user is redirected after a successful payment.
redirectUrls.failureUrl
string	yes	Where the user is redirected after a failure or cancel.
Create Payment

curl
JavaScript
C#
Java
PHP

curl --location 'https://pg-api-uat.zaincash.iq/api/v2/payment-gateway/transaction/init' \
--header 'Content-Type: application/json' \
--header 'Authorization: Bearer eyJraWQiOiJwYXltZW50LWdhdGV3YXkta2V5IiwiYWxnIjoiUlMyNTYifQ.eyJz.....' \
--data-raw '{
  "language": "en", // please make sure to choose the correct language based on your application locale.
  "externalReferenceId": "76554912-1810-4bf3-8b17-1bfc2c9fa6bf",
  "orderId": "moi-21323123vc",
  "serviceType": "Delivery",
  "amount": {
    "value": "3000",
    "currency": "IQD"
  },
  "customer": {
    "phone": "964XXXXXXXXXX"
  },
  "redirectUrls": {
    "successUrl": "https://merchant.com/payment/success",
    "failureUrl": "https://merchant.com/payment/failure"
  }
  }'
    
Request Body

Json
{
    "language": "en",
    "externalReferenceId": "d8594f04-cfcc-4fc3-b901-01513cc697bc",
    "orderId": "moi-21323123vc",
    "amount": {
        "value": "500",
        "currency": "IQD"
    },
     "customer": {
        "phone": "964XXXXXXXXXX"
    },
  "serviceType":"Delivery",
    "redirectUrls": {
        "successUrl": "https://www.example.merchant.com",
        "failureUrl": "https://www.example.merchant.com"
    }
}
Sample Response

Json
{
    "status": "SUCCESS",
    "transactionDetails": {
        "transactionId": "9da792d5-f818-4e98-9fb6-6c0b13902c8b",
        "externalReferenceId": "d8594f04-cfcc-4fc3-b901-01513cc697bc",
        "orderId": "moi-21323123vc",
        "amount": {
            "currency": "IQD",
            "value": 500
        }
    },
    "redirectUrl": "https://pg-api-uat.zaincash.iq/transaction/pay?id=transactionId&token=token_value",
    "expiryTime": "2025-12-22T08:04:27.402+00:00",
    "createdAt": "2025-12-22T07:49:28.540+00:00"
}
Always redirect the customer to the returned redirectUrl. Do not attempt to construct this URL manually.

Transaction Inquiry

Retrieve the latest status and details for a given payment transaction.

GET

/api/v2/payment-gateway/transaction/inquiry/{transactionId}


Scopes

payment:read

Parameter

Location

Type

Required

Description

transactionId
Path	string (UUID)	yes	Transaction ID from the init response.
Transaction Inquiry Request

curl
JavaScript
C#
Java
PHP
curl --location 'https://pg-api-uat.zaincash.iq/api/v2/payment-gateway/transaction/inquiry/4c13b31e-f736-4c37-abcf-aa61797431c7' \
    --header 'Authorization: Bearer eyJraWQiOiJwYXltZW50LWdhdGV3YXkta2V5IiwiYWxnIjoiUlMyNTYifQ.eyJzdWIiOiIxNDUwNjMxOTBmNGY0ZDkzYjYzMGQzNmU5NjA4MjNlMCIsImF1ZCI6IjE0NTA2MzE5MGY0ZjRkOTNiNjMwZDM2ZTk2MDgyM2UwIiwibmJmIjoxNzU5OTk3NDY4LCJzY29wZSI6WyJyZXZlcnNlOndyaXRlIiwicGF5bWVudDp3cml0ZSIsInBheW1lbnQ6cmVhZCIsInJldmVyc2U6cmVhZCJdLCJpc3MiOiJodHRwOi8vbG9jYWxob3N0OjgwODciLCJleHAiOjE3NjAwMDI0NjgsInRva2VuX3R5cGUiOiJCZWFyZXIiLCJpYXQiOjE3NTk5OTc0NjgsImp0aSI6ImIzMjFjZDUyLWY0MjUtNDkwNy1iMDcyLTJiM2IxYTcyNDJiZCJ9.G6gphiluYbgG3qi3uEUA_R7bbptlAUGVU6KhznNaRQjH15r35JpTJEvWffZsSmwvS7h66PrMKB7jyCZSZ6MCZZBqUR570pEJVF7ZdpBOeuBn8vgE3y9UYBuApV0h8PyXwuUt9x_pacu0xyUxTENikyfm8zo_3oAZ7W1E8eohcdfaVfQePP0iUr2dnqWL6C7--NQxnCQ5-odRx_umrmR0sd8Q2t2Jyzq1BCoz9nWzC5-svybEMrWhnWLI12J-G4wYyzs_OO1v1zf3BAT70RnwHwdHY3S0T_ziWP75EXPu6OfLgGY7Ym_6HaIAo9UNmYU4DwUCWwdP7sbIgMWEpG1xyw'
Sample Response

Json

{
    "status": "OTP_SENT",
    "transactionDetails": {
        "transactionId": "9da792d5-f818-4e98-9fb6-6c0b13902c8b",
        "operationId": null,
        "externalReferenceId": "d8594f04-cfcc-4fc3-b901-01513cc697bc",
        "orderId": "moi-21323123vc",
        "amount": {
            "currency": "IQD",
            "value": 500,
            "feeValue": 0
        }
    },
    "customer": {
        "phone": "964XXXXXXXXXX"
    },
    "timeStamps": {
        "expiryTime": "2025-12-22T08:04:27.403+00:00",
        "createdAt": "2025-12-22T07:49:28.540+00:00",
        "updatedAt": "2025-12-22T07:49:29.003+00:00",
        "completedAt": null
    }
}
Status Values

SUCCESS

FAILED

PENDING

OTP_SENT

CUSTOMER_AUTHENTICATION_REQUIRED

EXPIRED

REFUNDED

Reverse

Reverse a successfully completed transaction.

POST

/api/v2/payment-gateway/transaction/reverse


Scopes

reverse:write

Reverse/Refund Request

curl
JavaScript
C#
Java
PHP
curl --location 'https://pg-api-uat.zaincash.iq/api/v2/payment-gateway/transaction/reverse' \
--header 'Content-Type: application/json' \
--header 'Authorization: Bearer eyJraWQiOiJwYXltZW50LWdhdGV3YXkta2V5IiwiYWxnIjoiUlMyNTYifQ.eyJzdWIiOiIyYjU4MzhlNjVkODc0MmEwODI4NDMwOGZkZGQwNjQxNiIsImF1ZCI6IjJiNTgzOGU2NWQ4NzQyYTA4Mjg0MzA4ZmRkZDA2NDE2IiwibmJmIjoxNzU5NjUzNTEyLCJzY29wZSI6WyJyZXZlcnNlOndyaXRlIiwicGF5bWVudDp3cml0ZSIsInBheW1lbnQ6cmVhZCIsInJldmVyc2U6cmVhZCJdLCJpc3MiOiJodHRwOi8vdHYtc2l0LWFwcC56YWluY2FzaC5sb2NhbDo5MDEzIiwiZXhwIjoxNzU5NzM5OTEyLCJ0b2tlbl90eXBlIjoiQmVhcmVyIiwiaWF0IjoxNzU5NjUzNTEyLCJqdGkiOiJhZGU4MDU1OC1iM2JjLTQ3ZTMtOGM5Yy04NGZkYmZkZDFiMzgifQ.kQqXJCUz43yleJtqhXWEB8ZRUsoU8eLBY54G1I8BnX134LKH-BeeXnglIMtmdi-8SVrbBM0jJyl0kbSFx_lzNsewJSneeUVr36BsCD7wD6iHRfJj-pU4C1U5XOBIr5HP-TXbMwEylTxq0hJudgjpSODw4Qx5BYlI8D-_EG48kY3DMue4pVlp2iTTF1oNJY4v6L7yotBhuoKrKPUVhRvm8kg5nCcf2WOoJ6AXs5l2Be7xD--pKud4ik4u2cNXdym3X0ed5BfSuzGa9-0wPdbGVTugr4maj-HoB1DWhwqVdz3KkVKyOA1i8rVTy7nZ7Oqff0Kg7PPNrkvdoadopOzI6A' \
--data '{
  "transactionId": "69342191-d099-4ebc-8fba-d02556e10a0b",
  "reason": "example reason "
}
'
    
Request Body

Json
{
  "transactionId": "27011703-c800-40e2-b89e-951e7b4c5967",
  "reason": "example reason "
}
Field

Type

Required

Description

transactionId
string (UUID)	yes	The original successful transaction ID.
reason
string	yes	Business reason for initiating the reversal.
Sample Response

Json
{
    "id": 252,
    "customerId": 2096310,
    "operationId": 1253032369964175,
    "referenceId": "fd7b9848-2e9e-4ab3-b2d6-9b9d0bb2e2ba",
    "reversalReferenceId": "89da0fa7-040c-4101-9d90-66c301019312",
    "status": "COMPLETED",
    "customerMsisdn": "964XXXXXXXXXX",
    "merchantMsisdn": "964XXXXXXXXXX",
    "merchantId": "c2984b73ae4c48bb86ad5a19ce0ae362",
    "reason": "example reason ",
    "amount": 500,
    "createdAt": "2025-12-22T07:58:17.734+00:00",
    "updatedAt": "2025-12-22T07:58:17.734+00:00"
}
Redirect Callback

After the customer completes payment on the Payment Gateway page, ZainCash redirects the user back to your site.

Redirect URLs:

successUrl?token=JWT_TOKEN
failureUrl?token=JWT_TOKEN
Decoded Token Example

Json

{
  "eventType": "STATUS_CHANGED",
  "eventId": "812691e6-b433-4ffc-888c-d11e21c14994",
  "timestamp": "2023-10-27T10:15:30.000+00:00",
  "data": {
    "transactionId": "6fc49988-c618-4ee4-880b-d5a169693296",
    "merchantReferenceId": "6fc49988-c618-4ee4-880b-d5a169693296",
    "customerMsisdn": "9647XXXXXXXX",
    "orderId": "moi-21323123vc",
    "operationId": 1252738043970122,
    "serviceType": "Delivery",
    "language": "ar",
    "errorMessage": null,
    "previousStatus": "PENDING",
    "currentStatus": "SUCCESS",
    "amount": {
      "currency": "IQD",
      "value": 5000,
      "feeValue": 0
    }
  }
}
Verify the JWT signature using your API key and HS256 algorithm . Never trust callback data without verifying the token integrity .

Webhooks

Webhooks allow ZainCash to notify your backend whenever a transaction status changes or a refund completes. You configure a server-side notificationUrl that receives POST requests.

Prerequisites:

Contact the business team to set the webhook URL in our system and register it.
Please ensure that the webhook URL is separate from the success and failure URLs.
The configuration of the webhook only works in production and doesn't work in the test environment
Create WebHook

JavaScript
C#
Java
PHP
require("dotenv").config();
const express = require("express");
const jwt = require("jsonwebtoken");

const app = express();
app.use(express.json());
app.use(express.urlencoded({ extended: true }));

const ZAINCASH_SECRET = process.env.ZAINCASH_SECRET;

function verifyToken(token) {
  return jwt.verify(token, ZAINCASH_SECRET, { algorithms: ["HS256"] });
}

function processTransaction(payload) {
  const { status, orderid, id: txId, msg } = payload;
  console.log(`[ZainCash] orderid=${orderid} status=${status} txId=${txId}`);

  switch (status?.toLowerCase()) {
    case "success":
    case "completed":
      console.log(`✅ Payment confirmed — order ${orderid}`);
      break;
    case "failed":
      console.warn(`❌ Payment failed — order ${orderid}, reason: ${msg ?? "unknown"}`);
      break;
    case "pending":
      console.log(`⏳ Payment pending — order ${orderid}`);
      break;
    default:
      console.warn(`⚠️  Unknown status "${status}" for order ${orderid}`);
  }
}

app.post("/api/zaincash/webhook", (req, res) => {
  const token = req.body?.token;

  if (!token) {
    return res.status(400).json({ success: false, message: "Missing token in request body" });
  }

  try {
    const payload = verifyToken(token);
    processTransaction(payload);
    return res.status(200).json({ success: true });
  } catch (err) {
    if (err.name === "JsonWebTokenError" || err.name === "TokenExpiredError") {
      return res.status(401).json({ success: false, message: "Invalid or expired token" });
    }
    console.error("[ZainCash] Webhook error:", err);
    return res.status(500).json({ success: false, message: "Internal server error" });
  }
});

app.get("/api/zaincash/redirect", (req, res) => {
  const token = req.query?.token;

  if (!token) {
    return res.redirect("/payment?status=error&msg=missing_token");
  }

  try {
    const payload = verifyToken(token);
    processTransaction(payload);

    const { status, orderid, msg } = payload;
    if (status === "success" || status === "completed") {
      return res.redirect(`/payment/success?orderid=${orderid}`);
    }
    return res.redirect(`/payment/failed?orderid=${orderid}&reason=${msg ?? status}`);
  } catch {
    return res.redirect("/payment?status=error&msg=invalid_token");
  }
});

app.listen(process.env.PORT ?? 3000, () =>
  console.log("Server running on port", process.env.PORT ?? 3000)
);
Webhook Request

Json
{
"webhook_token":"webhook_token"
}
Method

Header

Description

POST
Content-Type: application/json	Body contains a webhook_token (JWT string).
Decoded JWT Example STATUS_CHANGED SUCCESS

Json

{
  "eventType": "STATUS_CHANGED",
  "eventId": "812691e6-b433-4ffc-888c-d11e21c14994",
  "timestamp": "2023-10-27T10:15:30.000+00:00",
  "data": {
    "transactionId": "6fc49988-c618-4ee4-880b-d5a169693296",
    "merchantReferenceId": "6fc49988-c618-4ee4-880b-d5a169693296",
    "customerMsisdn": "9647XXXXXXXX",
    "orderId": "moi-21323123vc",
    "operationId": 1252738043970122,
    "serviceType": "Delivery",
    "language": "ar",
    "errorMessage": null,
    "previousStatus": "PENDING",
    "currentStatus": "SUCCESS",
    "amount": {
      "currency": "IQD",
      "value": 5000,
      "feeValue": 0
    }
  }
}
Decoded JWT Example STATUS_CHANGED FAILED

Json

{
  "eventType": "STATUS_CHANGED",
  "eventId": "812691e6-b433-4ffc-888c-d11e21c14994",
  "timestamp": "2023-10-27T10:15:30.000+00:00",
  "data": {
    "transactionId": "6fc49988-c618-4ee4-880b-d5a169693296",
    "merchantReferenceId": "6fc49988-c618-4ee4-880b-d5a169693296",
    "customerMsisdn": "9647XXXXXXXX",
    "orderId": "moi-21323123vc",
    "operationId": 1252738043970122,
    "serviceType": "Delivery",
    "language": "ar",
    "errorMessage": 'Error!',
    "previousStatus": "PENDING",
    "currentStatus": "FAILED",
    "amount": {
      "currency": "IQD",
      "value": 5000,
      "feeValue": 0
    }
  }
}
Webhook Testing with cURL

Text
curl --location '<your-webhook-url>' --header 'Content-Type: application/json' --data '{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJldmVudFR5cGUiOiJTVEFUVVNfQ0hBTkdFRCIsImV2ZW50SWQiOiJ0ZXN0LWV2ZW50LTEyMzQ1IiwidGltZXN0YW1wIjoiMjAyNi0wMy0xN1QxMDowMDowMCswMDowMCIsImRhdGEiOnsidHJhbnNhY3Rpb25JZCI6InRlc3QtdHJhbnNhY3Rpb24tMTIzIiwibWVyY2hhbnRSZWZlcmVuY2VJZCI6InRlc3QtdHJhbnNhY3Rpb24tMTIzIiwiY3VzdG9tZXJNc2lzZG4iOiI5NjQ3ODAwMDAwMDAwIiwib3JkZXJJZCI6Im9yZGVyLTEyMyIsIm9wZXJhdGlvbklkIjoxMjM0NTY3ODkwLCJzZXJ2aWNlVHlwZSI6IkRlbGl2ZXJ5IiwibGFuZ3VhZ2UiOiJlbiIsImVycm9yTWVzc2FnZSI6bnVsbCwicHJldmlvdXNTdGF0dXMiOiJQRU5ESU5HIiwiY3VycmVudFN0YXR1cyI6IlNVQ0NFU1MiLCJhbW91bnQiOnsiY3VycmVuY3kiOiJJUUQiLCJ2YWx1ZSI6NTAwMCwiZmVlVmFsdWUiOjB9fX0.Owg3gE-w0txtc4CRTPej_HIdH0ffsoedLglw_eTHIbE"
}'
Use eventId for idempotency. If you receive the same eventId more than once, process it only once and return HTTP 200.

The configuration of the webhook only works in production and doesn’t work in this test environment.

The webhook payload gets sent by us only after the user finishes the last step in the payment and will be sent even if it’s a success or a failure you can check the body for it

Status & Params

Here are all the params that you need to know about the response and request:

Params

Field

Type

Required

Description

Location

grant_type
string	yes	Must be client_credentials.	/oauth2/token endpoint body request
client_id
string	yes	Your client ID.	/oauth2/token endpoint body request
client_secret
string	yes	Your client secret.	/oauth2/token endpoint body request
scope
string	yes	Space-separated scopes, e.g., payment:read payment:write.	/oauth2/token endpoint body request
language
string	yes	Language code: En, Ar, or Ku.	/api/v2/payment-gateway/transaction/init endpoint body request
externalReferenceId
string (UUID)	yes	Unique per request; use for idempotency and reconciliation.	/api/v2/payment-gateway/transaction/init endpoint body request
orderId
string	yes	Your internal order identifier.	/api/v2/payment-gateway/transaction/init endpoint body request
serviceType
string	yes	Service identifier (e.g., JAWS) provided by ZainCash.	/api/v2/payment-gateway/transaction/init endpoint body request
amount.value
string / number	yes	Transaction amount.	/api/v2/payment-gateway/transaction/init endpoint body request
amount.currency
string	yes	Must be IQD.	/api/v2/payment-gateway/transaction/init endpoint body request
customer.phone
string	optional	Customer phone in international format (e.g., 96477...).	/api/v2/payment-gateway/transaction/init endpoint body request
redirectUrls.successUrl
string	yes	Where the user is redirected after a successful payment.	/api/v2/payment-gateway/transaction/init endpoint body request
redirectUrls.failureUrl
string	yes	Where the user is redirected after a failure or cancel.	/api/v2/payment-gateway/transaction/init endpoint body request
transactionId
string (UUID)	yes	Transaction ID from the init response.	/api/v2/payment-gateway/transaction/inquiry/{transactionId} endpint body request
transactionId
string (UUID)	yes	The original successful transaction ID.	/api/v2/payment-gateway/transaction/reverse endpont response body
reason
string	yes	Business reason for initiating the reversal.	/api/v2/payment-gateway/transaction/reverse endpont response body
Status

Status Code

Description

SUCCESS
The final states of the transaction lifecycle . after a successful payment.
FAILED
The final states of the transaction lifecycle . after a failed payment attempt.
PENDING
Transaction created; awaiting next steps.
OTP_SENT
OTP delivered to the customer for authentication.
CUSTOMER_AUTHENTICATION_REQUIRED
Extra steps required (e.g., phone validation/fee computation pending or failed)
EXPIRED
Transaction exceeded its expiry time.
REFUNDED
The final states of the transaction lifecycle after a successful reversal/refund.
STATUS_CHANGED
Emitted when a transaction’s status after the payment process ends..
REFUND_COMPLETED
Emitted when a reversal/refund succeeds.
REFUND_FAILED
Emitted when a reversal fails.
Scopes

Scopes are basically the permissions needed for authorization to use the endpoints.

Scope

Description

payment:read
For writing privileges for payment gateway transaction processing
payment:write
For reading privileges for payment gateway transaction processing
reverse:write
For reversing a transaction
Status & Error Handling

Common error patterns you may encounter when integrating.

HTTP Status Codes

HTTP Code

Description

Notes

400
Bad Request	Invalid format (e.g., non-UUID transactionId) or missing fields.
401
Unauthorized	Missing / invalid bearer token, or expired token.
403
Forbidden	Transaction not owned by this merchant (e.g., PAYMENT_GATEWAY_UNAUTHORIZED).
404
Not Found	Transaction does not exist (PAYMENT_GATEWAY_TRANSACTION_NOT_FOUND).
Recommendations

Always log the HTTP status, error code, and full response body.
Use a correlation ID if provided by the gateway for debugging with ZainCash support.
Best Practices

Integration Guidelines

Idempotency

Use a unique externalReferenceId per payment attempt.
Do not reuse the same ID for multiple different orders.
On duplicate errors, perform an inquiry to determine the current status.
Security

Never expose your client_secret or API key in client-side code.
Verify all JWT tokens (redirect and webhook) using your API key and HS256.
Use HTTPS for all your redirect and notification URLs.
Resilience

Prefer webhooks as the primary signal for final status.
Use transactional retries with backoff when calling the gateway APIs.
Implement fallbacks with the inquiry API if webhooks are delayed.
Customer Wallet Management

The customer.phone field in the /transaction/init request is optional. ZainCash recommends the following approach for populating it.

Recommended flow

1. First-time payment — omit customer.phone

For a customer's first "Pay with ZainCash" transaction on your platform, send the /transaction/init request without the customer.phone field. ZainCash will prompt the customer to enter their wallet mobile number manually on the payment page.

2. Capture the wallet number from the success callback

Once the payment is completed successfully, ZainCash redirects the customer to your successUrl with a signed JWT token. After verifying the token with your API Secret Key, extract the payer wallet number from the token payload and store it against the customer's profile in your system.

3. Keep the stored wallet number up to date

After every successful transaction, update the stored wallet number with the new wallet number used in that transaction. Always rely on the Wallet Number from the latest successful payment — not only the first one — since a customer may switch wallets over time.

4. Subsequent payments — pass the latest saved wallet number

For every following transaction by the same customer, include the most recently saved wallet number in the customer.phone field of the /transaction/init request. The customer will not need to re-enter their wallet number on the payment page.

Use the wallet number captured from the latest successful ZainCash transaction rather than the mobile number registered on your platform, since the two may not be the same.

Brand Guidelines

Why this guideline:

To ensure consistent, professional, and user-friendly integration of “Pay with ZainCash” across web and mobile platforms.
To help merchants, developers, and designers implement ZainCash payment with correct branding, UI behavior, and user flow.
To maintain the integrity of ZainCash’s visual and interaction identity across partners.
Who should use this:

Front-end developers / integration engineers implementing ZainCash Pay.
UI/UX designers building checkout flows with ZainCash.
Product/marketing teams referencing ZainCash in UI, marketing, or payment-option contexts.
Brand Identity: Name, Logo & Visual Elements

Brand Name & Terminology:

Always refer to the service as “ZainCash” or “ZainCash Pay”. Do not abbreviate, translate, or alter the name (e.g., avoid “Z-Pay”, “ZCash wallet”, etc.).
In user-facing text, use consistent capitalization: capital “Z” and “C” — e.g., “Pay with ZainCash”.
The merchant must ensure that the correct language is used based on the customer’s language on the merchant platform. Supported languages are Arabic, English, and Kurdish.
Logo / Mark Usage:

Use only the official ZainCash logo or brand mark provided in their “Branding Guideline” asset pack.
Do not recolor, distort, stretch, or apply shadows or extra effects to the logo.
Maintain sufficient clear-space around the logo (no overlapping with other UI elements).
Do not combine the logo with other symbols or custom icons in a way that alters its appearance.
Download Links

Pay By ZainCash Plugin

ZainCash logs

Zain Font

Buttons

Button Style

Provide a dedicated button or UI element labeled “Pay with ZainCash” (or similar), when showing payment options.
Button styling should be clean, contrasting, and legible — consistent in padding, size, and color across pages (checkout, product page, etc.).
On light backgrounds — use a version of button that ensures text/logo readability; on dark or complex backgrounds — ensure contrast (e.g. light text/logo on dark background)
The button must trigger the actual ZainCash payment flow (not just a UI placeholder).
Primary Button

Button
Primary Button Disabled

Button
Secondary Button

Button
Secondary Button Disabled

Button
Button Sizing Guidelines

1. Minimum Button Size (Smallest Allowed)

To ensure readability + tapability (especially on mobile):
Minimum button size:

Width: 120px
Height:40px
Padding (Minimum):

Horizontal padding:16px
vertical padding:8–12px
Minimum Button Size:


2. Recommended Standard Button Size

This is the size most platforms use for wallet payments:
Standard recommended size:

Width: 200–240px
Height:48-54px
Padding (Recommended)

Horizontal padding:20-24px
vertical padding:12-14px
Standard Button Size


3. Largest Button Size (Maximum Recommended)

Avoid going too large — keep it balanced and professional.
Largest size:

Width:300–320px
Height:56–64px
Padding (Recommended)

Horizontal padding:24–32px
vertical padding:14-16px
Largest Button Size


Test Credentials

Test credentials used to make tests in a safe environment to try and handle all possible responses before going live.

Merchant

Please use the following merchant credentials to test transaction ID creation and status checks. Ensure you copy the credentials exactly as shown, without spaces, and note that the secret key must be entered as a single line.

#

MSISDN

Client ID

Client Secret

1
9647829744545	
758055f4a8044779a35f6ceb69f858b3


bibLCGTxVAig5To3OLLKPJQMlRR7Pefp


Customer

Please select one of the following customer test wallets to submit your transaction.

#

MSISDN

PIN

OTP

1
9647802999569	1111	111111
2
9647829744432	1111	111111
3
9647829744464	1111	111111
4
9647829744474	1111	111111
Going Live

Finished testing? Congratulations! You are ready to move to the live environment.

If you have already submitted your business request: Please contact your Business Development Account Manager to obtain your live credentials.

If you haven’t submitted a request yet, please complete your application by clicking here.

Merchant Dashboard

Once your business is registered in the ZainCash system, you will gain full access to our powerful reporting portal.

1- Check Your Email: You will automatically receive your login credentials from notification@zaincash.iq.

2- Log In: Access the Merchant Dashboard using your provided Merchant Password.

3- Take Control: Search transactions, view real-time history, and process reversals or partial refunds with one click.

FAQ

Didn’t find what you’re looking for?


Do we need IP authorization to access the test and production environments?

No, IP whitelisting is not required to access either the test or production environments.


What are the IPs of the test and production environments, so that we allow traffic between our systems.

There are no fixed IP addresses. Integration is performed through public URLs provided by ZainCash.


Do we need to verify the token you send to the success and failed URLs using the secret key?

Yes. The token must be verified using the same API Secret Key to ensure the authenticity and integrity of the callback.


Do we have to implement the webhook and rely on the redirection?

No, implementing the webhook is optional.


What are the possible values for the service_type in the init API?

The service_type field is merchant-defined. You may use any value that helps classify your services for internal tracking and reporting.


Is only a full refund possible or is there partial cancellation/refund available?

Please consult your ZainCash business representative to confirm the supported refund and cancellation options based on your commercial agreement.


How can we track the transactions?

Transactions can be tracked using the Inquiry API endpoint, which returns the current status and transaction details.


For how long can we check the status of a transaction using the inquiry API?

There is no time limit for checking transaction status. However, rate limits apply to the number of requests per second that can be sent to the API.


When do we get the Prod credentials?

Production credentials are issued after completion of the contractual and onboarding process with the ZainCash business team.


What are the wallet’s transaction limits?

Wallet transaction limits are defined in accordance with the limits set by the Central Bank of Iraq.


For how long does the transaction stay pending?

The pending duration depends on the globally configured transaction expiry time.


Can we use the payment gateway as an iframe inside our website?

No. Embedding the payment gateway inside an iframe is not supported.


How do I add or update my Webhook URL?

To add or update your Webhook URL, please contact the Zaincash Business Team. During the onboarding process, you will be asked to provide a Webhook URL. If you need to update it later, reach out to your Zaincash business representative.


Is the Webhook URL the same as the Success URL or Failure URL?

No, the Webhook URL you provide should be different from the Success URL and Failure URL.


When is the Webhook called?

The Webhook payload is sent after the user completes the final step of the payment. It is triggered for both successful and failed payments. You can check the payment status in the payload body.


Can I test the Webhook in the UAT environment?

No, the Webhook cannot be tested in the UAT environment.

Support

If you require any support or have questions not covered in this documentation, please open a request through our Ticket System.

Our team is available Sunday through Thursday, from 9:00 AM to 5:00 PM (Baghdad Time). We strive to respond to all inquiries within 12–24 hours on business days. While response times may occasionally be extended during peak periods, we are committed to providing you with a resolution as quickly as possible.

Please refer to the Service Level Agreement (SLA) table below for detailed response times related to Production issues only:

Severity Level

Definition

Response Time

Resolution Time

P1 High
Complete Outage (Payment Gateway API down). Major degradation impacting a large portion of payments, but not a total outage. Merchant dashboard down, cash disbursement.	<1 Hour	<8 Hours
P2 Medium
Limited disruption affecting a subset of transactions or non-financial functions (e.g., merchant dashboard reports not updating real-time, no financial loss).	<4 Hours	<24 Hours
P3 Low
Minor/localized issues with no significant business or financial impact (e.g., single refund delayed, individual user cases) or settlement delayed by 1 day.	<1 Business Day	<10 Business Days
Note: While we aim to provide the best support possible, please keep in mind that it only extends to resolving technical matters. We will not implement the integration for you or fix unrelated issues. However, we will suggest fixes and provide guidance.