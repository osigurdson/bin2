import { withAuth } from "@workos-inc/authkit-nextjs";
import { NextResponse } from "next/server";
import type { ContactRequest } from "@/api/contact/types";

const nthesisContactBaseUrl = "https://nthesis.ai/api/v1/items";
const maxNameLength = 200;
const maxEmailLength = 254;
const maxMessageLength = 10_000;
const maxTopicLength = 64;
const maxSourceLength = 64;
const rateLimitWindowMs = 10 * 60 * 1000;
const rateLimitMaxRequests = 5;
const allowedTopics = new Set(['Bug report', 'Question', 'Feature request', 'Other']);
const contactRateLimitStore = new Map<string, { count: number; resetAt: number }>();

function validateEmail(value: string) {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value);
}

function getHeaderValue(req: Request, name: string) {
  return req.headers.get(name)?.trim() ?? '';
}

function getClientIdentifier(req: Request, userEmail: string) {
  if (userEmail) {
    return `user:${userEmail.toLowerCase()}`;
  }

  const forwardedFor = getHeaderValue(req, 'x-forwarded-for');
  if (forwardedFor) {
    return `ip:${forwardedFor.split(',')[0]?.trim() ?? forwardedFor}`;
  }

  const realIp = getHeaderValue(req, 'x-real-ip');
  if (realIp) {
    return `ip:${realIp}`;
  }

  const cloudflareIp = getHeaderValue(req, 'cf-connecting-ip');
  if (cloudflareIp) {
    return `ip:${cloudflareIp}`;
  }

  const userAgent = getHeaderValue(req, 'user-agent');
  return `fallback:${userAgent || 'unknown'}`;
}

function consumeRateLimit(identifier: string) {
  const now = Date.now();

  for (const [key, entry] of contactRateLimitStore.entries()) {
    if (entry.resetAt <= now) {
      contactRateLimitStore.delete(key);
    }
  }

  const current = contactRateLimitStore.get(identifier);
  if (!current || current.resetAt <= now) {
    contactRateLimitStore.set(identifier, {
      count: 1,
      resetAt: now + rateLimitWindowMs,
    });
    return { allowed: true, retryAfterSeconds: Math.ceil(rateLimitWindowMs / 1000) };
  }

  if (current.count >= rateLimitMaxRequests) {
    return {
      allowed: false,
      retryAfterSeconds: Math.max(1, Math.ceil((current.resetAt - now) / 1000)),
    };
  }

  current.count += 1;
  contactRateLimitStore.set(identifier, current);
  return {
    allowed: true,
    retryAfterSeconds: Math.max(1, Math.ceil((current.resetAt - now) / 1000)),
  };
}

export async function POST(req: Request) {
  let body: Partial<ContactRequest>;

  try {
    body = (await req.json()) as Partial<ContactRequest>;
  } catch {
    return NextResponse.json({ ok: false, error: 'Bad Request' }, { status: 400 });
  }

  const rawName = typeof body.name === 'string' ? body.name.trim() : '';
  const rawEmail = typeof body.email === 'string' ? body.email.trim() : '';
  const rawMessage = typeof body.message === 'string' ? body.message.trim() : '';
  const rawTopic = typeof body.topic === 'string' ? body.topic.trim() : '';
  const rawSource = typeof body.source === 'string' ? body.source.trim() : '';

  if (
    rawName.length > maxNameLength ||
    rawEmail.length > maxEmailLength ||
    rawMessage.length > maxMessageLength ||
    rawTopic.length > maxTopicLength ||
    rawSource.length > maxSourceLength
  ) {
    return NextResponse.json({ ok: false, error: 'Bad Request' }, { status: 400 });
  }

  const { user } = await withAuth();
  const userEmail = user?.email?.trim() ?? '';
  const userName = user?.firstName?.trim() || userEmail || rawName;
  const name = userEmail ? userName : rawName;
  const email = userEmail || rawEmail;
  const message = rawMessage;
  const source = userEmail ? 'dashboard-help-feedback' : 'public-contact';
  const topic = userEmail
    ? (allowedTopics.has(rawTopic) ? rawTopic : 'Question')
    : '';

  if (!name || !message) {
    return NextResponse.json({ ok: false, error: 'Bad Request' }, { status: 400 });
  }
  if (email && !validateEmail(email)) {
    return NextResponse.json({ ok: false, error: 'Bad Request' }, { status: 400 });
  }

  const rateLimit = consumeRateLimit(getClientIdentifier(req, userEmail));
  if (!rateLimit.allowed) {
    return NextResponse.json(
      { ok: false, error: 'Too many requests. Please wait and try again.' },
      {
        status: 429,
        headers: {
          'Retry-After': `${rateLimit.retryAfterSeconds}`,
        },
      },
    );
  }

  const contactStoreId = process.env.NTHESIS_CONTACT_STORE_ID?.trim();
  const contactApiKey = process.env.NTHESIS_CONTACT_API_KEY?.trim();
  if (!contactStoreId || !contactApiKey) {
    return NextResponse.json({ ok: false, error: 'Contact form not configured' }, { status: 500 });
  }

  const backendUrl = `${nthesisContactBaseUrl}?storeId=${encodeURIComponent(contactStoreId)}`;
  try {
    const res = await fetch(backendUrl, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-API-Key': contactApiKey,
      },
      body: JSON.stringify({
        content: JSON.stringify({
          name,
          email,
          message,
          ...(topic ? { topic } : {}),
          source,
        }),
      }),
    });

    if (!res.ok && res.status !== 409) {
      return NextResponse.json({ ok: false, error: 'Failed to send message' }, { status: 502 });
    }
  } catch {
    return NextResponse.json({ ok: false, error: 'Failed to send message' }, { status: 502 });
  }

  return NextResponse.json({ ok: true });
}
