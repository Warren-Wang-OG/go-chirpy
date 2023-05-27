# Chirpy Web Server
RESTful API

## Available Endpoints:

### `POST /api/users` - Create a new User

Request Body:
```json
{
    "email": "example@gmail.com",
    "password": "notasecurepassword123"
}
```

Response Body:
```json
{
    "id": 1,
    "email": "example@gmail.com"
}
```

### `PUT /api/users` - Update an existing User, need to be authenticated already

Headers needed:
`Authorization: Bearer <token>`

Request Body:
```json
{
    "email": "newemailexample@gmail.com",
    "password": "atotallysecurepassword389"
}
```

Response Body:
```json
{
    "id": 1,
    "email": "example@gmail.com"
}
```

### `POST /api/login` - Authenticate a User 

Request Body:
```json
{
    "email": "newemailexample@gmail.com",
    "password": "atotallysecurepassword389"
}
```

Response Body:
```json
{
    "id": 1,
    "email": "example@gmail.com",
    "is_chirpy_red": false,
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJjaGlycHktYWNjZXNzIiwic3ViIjoiMSIsImV4cCI6MTY4NTIyNDQ4MiwiaWF0IjoxNjg1MjIwODgyfQ.zvUD8YgYbp17RUnbuMydhyBlTcuAQcL4Gt4vkDSqCOE",
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJjaGlycHktcmVmcmVzaCIsInN1YiI6IjEiLCJleHAiOjE2OTA0MDQ4ODIsImlhdCI6MTY4NTIyMDg4Mn0.yfaZs2dhNj9AQxkJbWoCK2NuLsfjWZTEXBaC-4QFkiY"
}
```

### `POST /api/polka/webhooks` - Upgrade a user to "Chirpy Red" 

Headers required:
`Authorization: ApiKey <key>`

Request Body:
```json
{
    "event": "user.upgraded",
    "data": {
        "user_id": 1
    }
}
```

Response Body:
```json
null
```
Response Code: `200`

If `event` is anything other than `user.upgraded`, will not upgrade user and return `200`. If request does not provide or has an incorrect polka apikey, response code will be `401`. If response contains `user.upgraded` and has a valid polka apikey, will return code `200`.

### `POST /api/refresh` - Use a (non-expired, non-revoked) refresh token to get a new access token

Headers Required:
`Authorization: Bearer <refresh-token>`

No Request Body expected

Response Body:
```json
{
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJjaGlycHktYWNjZXNzIiwic3ViIjoiMSIsImV4cCI6MTY4NTIyNDQ4MiwiaWF0IjoxNjg1MjIwODgyfQ.zvUD8YgYbp17RUnbuMydhyBlTcuAQcL4Gt4vkDSqCOE"
}
```

### `POST /api/revoke` - Revoke the given refresh token

Headers Required:
`Authorization: Bearer <refresh-token>`

No Request Body expected

Response Body:
```json
null
``` 
Response Code: `200`

If response code is not `200`, then you will get an error and a corresponding code instead.

### `POST /api/chirps` - Create a Chirp (post), authenticated endpoint
Chirps can only be created by Users that have been created and logged in (requires access token). Chirps' contents must be 140 characters or less. If they contain the words `["kerfuffle", "sharbert", "fornax"]` they will be censored with `****`.

Headers Required:
`Authorization: Bearer <token>`

Request Body:
```json
{
    "body": "this is an example chirp~"
}
```

Response Body:
```json
{
    "id": 1, 
    "body": "this is an example chirp~",
    "author_id": 1
}
```

`id` is the chirp id and `author_id` is the id of the corresponding user who made the chirp.


### `GET /api/chirps` - Get all chirps
- Accepts an optional `author` parameter (e.g. `GET http://localhost:8080/api/chirps?author_id=1` ) and if present, returns all the chirps by the specified author. If author user doesn't exist, returns an empty list.

Response Body:
```json
[
  {
    "id": 1,
    "body": "this is my first chirp!!",
    "author_id": 1
  },
  {
    "id": 2,
    "body": "this is my second **** chirp!",
    "author_id": 1
  }
]
```

Chirps are ordered by `id` in ascending order.

### `GET /api/chirps{id}` - Get a single Chirp by its `id`

Example request: `GET localhost:8080/api/chirps/2`

Response Body:
```json
{
  "id": 2,
  "body": "this is my second **** chirp!",
  "author_id": 1
}
```

### `DELETE /api/chirps/{chirpID}` - Delete a chirp by its `id`, authenticated endpoint

You must provide an access token and you can only delete chirps that you have created.

Example request: `DELETE localhost:8080/api/chirps/2`

Headers Required:
`Authorization: Bearer <token>`

No Request Body expected.

Response Body:
```json
null
```
Response Code: `200`

As always, if there is some error, you will be given an appropriate response code and error message in the body.

### `GET /api/healthz` - Readiness Endpoint

Response Body:
```
OK
```
Response Code: `200`

If receive anything else, server is not up.

### `GET /admin/metrics` - Get how many times `/` has been served

Response Body:
```html
<html>

  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited 1 times!</p>
  </body>

</html>
```

The template of the html page can be changed in the file `/admin/metrics/template.html`

## Fileserver

### `GET /` - the main landing page

Response Body:
```html
<html>

  <body>
    <h1>Welcome to Chirpy</h1>
  </body>

</html>
```

The template of the html page can be changed in the file `/index.html`

### `GET /assets/logo.png` - a png of a bird

![](./assets/logo.png)

## Environment variables
You need a `.env` that contains
```
JWT_SECRET=<your-super-secret-at-least-32-char-long-key>
POLKA_KEY=<super-secret-api-key>
```

Notes:
- "Chirpy Red" is a fictitious elevated subscription tier that users get upgraded to from 
- *Project written following the outlines of "Learn Web Servers" on [boot.dev](https://boot.dev/tracks/backend)*