import { NextResponse } from "next/server";
import type { ContactRequest } from "@/api/contact/types";

const nthesisContactBaseUrl = "https://nthesis.ai/api/v1/items";

export async function POST(req: Request) {
  let body: Partial<ContactRequest>;

  try {
    body = (await req.json()) as Partial<ContactRequest>;
  } catch {
    return NextResponse.json({ ok: false, error: 'Bad Request' }, { status: 400 });
  }

  const name = body.name?.trim() ?? '';
  const email = body.email?.trim() ?? '';
  const message = body.message?.trim() ?? '';

  if (!name || !message) {
    return NextResponse.json({ ok: false, error: 'Bad Request' }, { status: 400 });
  }

  const contactStoreId = process.env.NTHESIS_CONTACT_STORE_ID?.trim();
  const contactApiKey = process.env.NTHESIS_CONTACT_API_KEY?.trim();
  if (!contactStoreId || !contactApiKey) {
    return NextResponse.json({ ok: false, error: 'Contact form not configured' }, { status: 500 });
  }

  const backendUrl = `${nthesisContactBaseUrl}?storeId=${encodeURIComponent(contactStoreId)}`;
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
      }),
    }),
  });

  if (!res.ok && res.status !== 409) {
    return NextResponse.json({ ok: false }, { status: res.status });
  }

  return NextResponse.json({ ok: true });
}
