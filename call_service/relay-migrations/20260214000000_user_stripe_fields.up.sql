ALTER TABLE users ADD COLUMN stripe_customer_id TEXT;
ALTER TABLE users ADD COLUMN subscription_status TEXT DEFAULT 'none';
ALTER TABLE users ADD COLUMN subscription_stripe_id TEXT;
ALTER TABLE users ADD COLUMN credit_minutes INTEGER DEFAULT 0;
