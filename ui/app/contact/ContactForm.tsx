'use client';

import type { FormEvent } from "react";
import { useEffect, useState } from "react";
import type { ContactRequest } from "@/api/contact/types";

function validateEmail(value: string) {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value);
}

async function submitContactRequest(payload: ContactRequest) {
  const res = await fetch('/api/contact', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });

  let data: { ok?: boolean; error?: string } | null = null;
  try {
    data = (await res.json()) as { ok?: boolean; error?: string };
  } catch {
    data = null;
  }

  if (!res.ok) {
    if (res.status === 409) {
      return;
    }
    throw new Error(typeof data?.error === 'string' ? data.error : 'Failed to send');
  }

  if (!data?.ok) {
    throw new Error('Failed to send');
  }
}

type ContactFormProps = {
  authenticatedIdentity?: {
    name: string;
    email: string;
  } | null;
};

const helpTopics = [
  'Bug report',
  'Question',
  'Feature request',
  'Other',
] as const;

export default function ContactForm({ authenticatedIdentity }: ContactFormProps) {
  const isAuthenticatedFeedback = !!authenticatedIdentity?.email;
  const [name, setName] = useState(authenticatedIdentity?.name ?? '');
  const [email, setEmail] = useState(authenticatedIdentity?.email ?? '');
  const [message, setMessage] = useState('');
  const [topic, setTopic] = useState<(typeof helpTopics)[number]>('Bug report');
  const [error, setError] = useState('');
  const [sent, setSent] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);

  useEffect(() => {
    if (!sent) {
      return;
    }

    const timer = setTimeout(() => setSent(false), 2500);
    return () => clearTimeout(timer);
  }, [sent]);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();

    const trimmedName = name.trim();
    const trimmedEmail = email.trim();
    const trimmedMessage = message.trim();

    if (!trimmedName) {
      setError('Name is required.');
      return;
    }

    if (trimmedEmail && !validateEmail(trimmedEmail)) {
      setError('Please enter a valid email address.');
      return;
    }

    if (!trimmedMessage) {
      setError('Please enter a message.');
      return;
    }

    setError('');
    setIsSubmitting(true);

    try {
      await submitContactRequest({
        name: trimmedName,
        email: trimmedEmail,
        message: trimmedMessage,
        topic: isAuthenticatedFeedback ? topic : undefined,
        source: isAuthenticatedFeedback ? 'dashboard-help-feedback' : 'public-contact',
      });
      setSent(true);
      setMessage('');
      if (!isAuthenticatedFeedback) {
        setName('');
        setEmail('');
      }
    } catch (error) {
      setError(error instanceof Error ? error.message : 'Error sending message.');
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <form
      className="mx-auto flex w-full max-w-xl flex-col gap-5 border border-base-200 bg-base-100 px-5 py-6 sm:px-8 sm:py-8"
      onSubmit={handleSubmit}
    >
      <div className="flex flex-col gap-2">
        <p className="text-xs uppercase tracking-[2px] text-base-content/40">
          {isAuthenticatedFeedback ? 'Help & Feedback' : 'Contact'}
        </p>
        <h1 className="text-3xl font-bold">
          {isAuthenticatedFeedback ? 'Need help with bin2?' : 'Get in touch'}
        </h1>
        <p className="text-sm leading-6 text-base-content/60">
          {isAuthenticatedFeedback
            ? 'Tell us what is broken, confusing, or missing. We will get back to you right away.'
            : "Questions, feedback, or something not working right? Send a note and we'll take a look."}
        </p>
        {isAuthenticatedFeedback && (
          <p className="text-xs text-base-content/50">
            Signed in as {authenticatedIdentity?.email}
          </p>
        )}
      </div>

      {!isAuthenticatedFeedback && (
        <label className="flex flex-col gap-2 text-sm">
          <span className="text-base-content/70">Name</span>
          <input
            type="text"
            value={name}
            onChange={(event) => setName(event.target.value)}
            placeholder="Your name"
            className="border border-base-300 bg-transparent px-4 py-3 outline-none transition-colors focus:border-base-content"
          />
        </label>
      )}

      {!isAuthenticatedFeedback && (
        <label className="flex flex-col gap-2 text-sm">
          <span className="text-base-content/70">Email (optional)</span>
          <input
            type="email"
            value={email}
            onChange={(event) => setEmail(event.target.value)}
            placeholder="you@example.com"
            className="border border-base-300 bg-transparent px-4 py-3 outline-none transition-colors focus:border-base-content"
          />
        </label>
      )}

      {isAuthenticatedFeedback && (
        <label className="flex flex-col gap-2 text-sm">
          <span className="text-base-content/70">What is this about?</span>
          <select
            value={topic}
            onChange={(event) => setTopic(event.target.value as (typeof helpTopics)[number])}
            className="border border-base-300 bg-transparent px-4 py-3 outline-none transition-colors focus:border-base-content font-[inherit]"
          >
            {helpTopics.map((option) => (
              <option key={option} value={option}>
                {option}
              </option>
            ))}
          </select>
        </label>
      )}

      <label className="flex flex-col gap-2 text-sm">
        <span className="text-base-content/70">{isAuthenticatedFeedback ? 'Details' : 'Message'}</span>
        <textarea
          value={message}
          onChange={(event) => setMessage(event.target.value)}
          placeholder={isAuthenticatedFeedback ? 'Describe the issue, question, or idea.' : 'How can we help?'}
          className="min-h-36 resize-none border border-base-300 bg-transparent px-4 py-3 outline-none transition-colors focus:border-base-content"
        />
      </label>

      <div className="flex min-h-5 items-center justify-between gap-4 text-sm">
        <div>
          {error && <p className="text-error">{error}</p>}
          {!error && sent && <p className="text-success">Message sent successfully.</p>}
        </div>
        <button
          type="submit"
          disabled={isSubmitting}
          className="inline-flex items-center justify-center border border-base-content bg-base-content px-4 py-2 text-base-100 transition-colors hover:bg-base-content/80 hover:border-base-content/80 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {isSubmitting ? 'Sending...' : isAuthenticatedFeedback ? 'Send feedback' : 'Send message'}
        </button>
      </div>
    </form>
  );
}
