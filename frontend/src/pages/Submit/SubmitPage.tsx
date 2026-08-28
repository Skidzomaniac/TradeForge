import { useEffect, useRef, useState } from 'react';
import toast from 'react-hot-toast';
import { useNavigate } from 'react-router-dom';
import { Button, Card, Badge, statusVariant } from '@/components/UI';
import { createSubmission, createTest, getSubmission } from '@/lib/api';
import { getApiKey } from '@/lib/auth';
import type { Submission } from '@/types/leaderboard';

const LANGS = ['cpp', 'rust', 'go', 'python'];

export function SubmitPage() {
  const apiKey = getApiKey() || '';
  const navigate = useNavigate();
  const [file, setFile] = useState<File | null>(null);
  const [language, setLanguage] = useState('cpp');
  const [submissionId, setSubmissionId] = useState<string | null>(null);
  const [submission, setSubmission] = useState<Submission | null>(null);
  const [uploading, setUploading] = useState(false);
  const pollRef = useRef<number | null>(null);

  useEffect(() => {
    if (!submissionId) return;
    const poll = async () => {
      try {
        const s = await getSubmission(submissionId, apiKey);
        setSubmission(s);
        if (s.status === 'ready' || s.status === 'failed') {
          if (pollRef.current) window.clearInterval(pollRef.current);
        }
      } catch {
        /* ignore */
      }
    };
    poll();
    pollRef.current = window.setInterval(poll, 2000);
    return () => {
      if (pollRef.current) window.clearInterval(pollRef.current);
    };
  }, [submissionId, apiKey]);

  const onFile = (f: File) => {
    setFile(f);
    const ext = f.name.split('.').pop()?.toLowerCase();
    if (ext === 'cpp') setLanguage('cpp');
  };

  const upload = async () => {
    if (!file) return;
    setUploading(true);
    try {
      const res = await createSubmission(file, language, apiKey);
      setSubmissionId(res.submission_id);
      toast.success('Build queued');
    } catch (e) {
      toast.error((e as Error).message);
    } finally {
      setUploading(false);
    }
  };

  const startTest = async () => {
    if (!submissionId) return;
    try {
      const res = await createTest({ submission_id: submissionId, duration_seconds: 30, bot_count: 10 }, apiKey);
      toast.success('Test started');
      navigate(`/results/${res.test_id}`);
    } catch (e) {
      toast.error((e as Error).message);
    }
  };

  return (
    <div className="p-8 max-w-2xl">
      <h1 className="text-2xl font-bold mb-4 font-mono text-accent-green">Submit Your Order Book</h1>

      <Card className="mb-4">
        <label
          className="block border-2 border-dashed border-surface-hover rounded p-8 text-center cursor-pointer hover:border-accent-green transition-colors"
          onDragOver={(e) => e.preventDefault()}
          onDrop={(e) => {
            e.preventDefault();
            if (e.dataTransfer.files[0]) onFile(e.dataTransfer.files[0]);
          }}
        >
          <input
            type="file"
            accept=".zip,.cpp,.rs,.go"
            className="hidden"
            onChange={(e) => e.target.files && onFile(e.target.files[0])}
          />
          <span className="font-mono text-gray-400">
            {file ? `${file.name} (${(file.size / 1024).toFixed(0)} KB)` : 'Drop a .zip or .cpp file, or click to browse'}
          </span>
        </label>

        <div className="flex gap-4 mt-4 items-center">
          <select
            value={language}
            onChange={(e) => setLanguage(e.target.value)}
            className="bg-surface-primary border border-surface-hover rounded p-2 text-gray-200 font-mono"
          >
            {LANGS.map((l) => (
              <option key={l} value={l}>
                {l}
              </option>
            ))}
          </select>
          <Button onClick={upload} disabled={!file} loading={uploading}>
            Upload
          </Button>
        </div>
      </Card>

      {submission && (
        <Card>
          <div className="flex items-center gap-3 mb-3">
            <span className="font-mono text-gray-400">Build status:</span>
            <Badge variant={statusVariant(submission.status)}>{submission.status}</Badge>
          </div>
          {submission.error_log && (
            <pre className="bg-surface-primary p-3 rounded text-accent-red text-xs overflow-x-auto max-h-48">
              {submission.error_log}
            </pre>
          )}
          {submission.status === 'ready' && (
            <Button onClick={startTest} className="mt-2">
              Run Test
            </Button>
          )}
        </Card>
      )}
    </div>
  );
}
