-- tenants
INSERT INTO
    tenants(created_at, updated_at, id, email, profile)
VALUES
    (
        now(), now(), '1efbccc0-87be-4672-a768-9d16c0123541', 'osio-developer1@email.com', 'free'
    ),
    (
        now(), now(), '0d19928e-ef61-46fd-9bdc-71d1ecbce2c7', 'osio-developer2@email.com', 'free'
    )
;
