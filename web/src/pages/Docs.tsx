import { useEffect, useMemo, useState } from 'react';
import { Card, Col, Row, Space, Typography, Divider, Button, List, Tag, Empty, Spin, message } from 'antd';
import {
  BookOutlined,
  ReadOutlined,
  BranchesOutlined,
  CloudOutlined,
  RocketOutlined,
} from '@ant-design/icons';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import api from '@/api/client';

const { Title, Paragraph, Text } = Typography;

type DocEntry = {
  id: string;
  title: string;
  path: string;
  category: string;
  description: string;
  available: boolean;
};

type DocContent = {
  id: string;
  title: string;
  path: string;
  category: string;
  description: string;
  content: string;
};

function resolveDocIdFromHref(href: string, docs: DocEntry[]): string | null {
  const cleanHref = href.split('#')[0].split('?')[0];
  if (!cleanHref.toLowerCase().endsWith('.md')) return null;

  const fileName = cleanHref.split('/').filter(Boolean).pop();
  if (!fileName) return null;

  const byPath = docs.find((d) => d.path.toLowerCase().endsWith(fileName.toLowerCase()));
  if (byPath) return byPath.id;

  const inferredId = fileName.replace(/\.md$/i, '').replace(/_/g, '-').toLowerCase();
  const byId = docs.find((d) => d.id.toLowerCase() === inferredId);
  return byId ? byId.id : null;
}

export default function Docs() {
  const [docs, setDocs] = useState<DocEntry[]>([]);
  const [selectedDocId, setSelectedDocId] = useState<string>('');
  const [selectedDoc, setSelectedDoc] = useState<DocContent | null>(null);
  const [loadingList, setLoadingList] = useState(true);
  const [loadingDoc, setLoadingDoc] = useState(false);
  const [docError, setDocError] = useState<string>('');

  useEffect(() => {
    const loadDocs = async () => {
      setLoadingList(true);
      try {
        const items = await api.getDocs();
        const normalizedItems = Array.isArray(items) ? items : [];
        const availableDocs = normalizedItems.filter((d) => d && typeof d.id === 'string' && d.available);
        setDocs(availableDocs);

        const preferred = availableDocs.find((d) => d.id === 'documentation-index') || availableDocs[0];
        if (preferred) {
          setSelectedDocId(preferred.id);
        }
      } catch (error) {
        message.error('Failed to load documentation list');
        setDocs([]);
      } finally {
        setLoadingList(false);
      }
    };

    loadDocs();
  }, []);

  useEffect(() => {
    if (!selectedDocId) return;

    const loadDoc = async () => {
      setLoadingDoc(true);
      setDocError('');
      try {
        const doc = await api.getDoc(selectedDocId);
        if (!doc || typeof doc !== 'object' || typeof doc.content !== 'string') {
          throw new Error('Invalid documentation payload');
        }
        setSelectedDoc(doc as DocContent);
      } catch (error) {
        setDocError('Unable to render this document from API response. Please refresh and try again.');
        message.error('Failed to load document content');
        setSelectedDoc(null);
      } finally {
        setLoadingDoc(false);
      }
    };

    loadDoc();
  }, [selectedDocId]);

  const categories = useMemo(() => {
    const grouped: Record<string, DocEntry[]> = {};
    for (const doc of docs) {
      if (!grouped[doc.category]) grouped[doc.category] = [];
      grouped[doc.category].push(doc);
    }
    return Object.entries(grouped).sort(([a], [b]) => a.localeCompare(b));
  }, [docs]);

  return (
    <Space direction="vertical" size="large" style={{ width: '100%' }}>
      <Card style={{ background: '#1f1f1f', border: '1px solid #303030' }}>
        <Space align="start">
          <BookOutlined style={{ fontSize: 24, color: '#DC382D', marginTop: 2 }} />
          <div>
            <Title level={3} style={{ margin: 0, color: 'rgba(255,255,255,0.92)' }}>
              RedisMeter Documentation
            </Title>
            <Paragraph style={{ marginTop: 8, marginBottom: 0, color: 'rgba(255,255,255,0.75)' }}>
              Full project documentation is readable directly in the UI. Use the left panel to select a document.
            </Paragraph>
          </div>
        </Space>
      </Card>

      <Row gutter={[16, 16]}>
        <Col xs={24} lg={8}>
          <Card title={<><ReadOutlined /> Beginner Path</>} style={{ background: '#1f1f1f', border: '1px solid #303030' }}>
            <List
              size="small"
              dataSource={[
                'Read the Documentation Hub',
                'Follow Beginner UI Workflow',
                'Run two controlled benchmarks',
                'Create baseline and compare',
              ]}
              renderItem={(item) => <List.Item style={{ color: 'rgba(255,255,255,0.75)' }}>{item}</List.Item>}
            />
          </Card>
        </Col>
        <Col xs={24} lg={8}>
          <Card title={<><BranchesOutlined /> Core Concepts</>} style={{ background: '#1f1f1f', border: '1px solid #303030' }}>
            <Space wrap>
              <Tag color="blue">Benchmark</Tag>
              <Tag color="purple">Baseline</Tag>
              <Tag color="gold">Compare</Tag>
              <Tag color="green">Analysis</Tag>
              <Tag color="cyan">Job Lifecycle</Tag>
              <Tag color="magenta">Runner</Tag>
            </Space>
            <Paragraph style={{ marginTop: 12, color: 'rgba(255,255,255,0.75)' }}>
              These concepts are explained in detail with workflow examples and interpretation guidance.
            </Paragraph>
          </Card>
        </Col>
        <Col xs={24} lg={8}>
          <Card title={<><CloudOutlined /> Cloud and Azure</>} style={{ background: '#1f1f1f', border: '1px solid #303030' }}>
            <Paragraph style={{ color: 'rgba(255,255,255,0.75)' }}>
              Understand control plane vs data plane, remote runner orchestration, AMR target modes,
              and networking design for reliable cloud tests.
            </Paragraph>
            <Button
              type="primary"
              icon={<RocketOutlined />}
              onClick={() => setSelectedDocId('azure-deployment-design')}
            >
              Open Azure Guide
            </Button>
          </Card>
        </Col>
      </Row>

      <Row gutter={[16, 16]}>
        <Col xs={24} lg={8} xl={7}>
          <Card title="Documentation Library" style={{ background: '#1f1f1f', border: '1px solid #303030', height: '100%' }}>
            {loadingList ? (
              <div style={{ textAlign: 'center', padding: '24px 0' }}>
                <Spin />
              </div>
            ) : docs.length === 0 ? (
              <Empty description="No documentation files available" image={Empty.PRESENTED_IMAGE_SIMPLE} />
            ) : (
              <Space direction="vertical" style={{ width: '100%' }} size="middle">
                {categories.map(([category, items]) => (
                  <div key={category}>
                    <Text strong style={{ color: 'rgba(255,255,255,0.85)' }}>{category}</Text>
                    <List
                      size="small"
                      dataSource={items}
                      renderItem={(doc) => (
                        <List.Item
                          style={{
                            borderRadius: 6,
                            padding: '8px 10px',
                            cursor: 'pointer',
                            background: selectedDocId === doc.id ? 'rgba(220,56,45,0.12)' : 'transparent',
                            border: selectedDocId === doc.id ? '1px solid rgba(220,56,45,0.4)' : '1px solid transparent',
                          }}
                          onClick={() => setSelectedDocId(doc.id)}
                        >
                          <Space direction="vertical" size={2} style={{ width: '100%' }}>
                            <Text strong style={{ color: 'rgba(255,255,255,0.9)' }}>{doc.title}</Text>
                            <Text style={{ color: 'rgba(255,255,255,0.6)' }}>{doc.description}</Text>
                          </Space>
                        </List.Item>
                      )}
                    />
                  </div>
                ))}
              </Space>
            )}
          </Card>
        </Col>

        <Col xs={24} lg={16} xl={17}>
          <Card
            title={selectedDoc?.title || 'Document Viewer'}
            extra={selectedDoc ? <Tag>{selectedDoc.category}</Tag> : undefined}
            style={{ background: '#1f1f1f', border: '1px solid #303030' }}
          >
            {loadingDoc ? (
              <div style={{ textAlign: 'center', padding: '48px 0' }}>
                <Spin size="large" />
              </div>
            ) : !selectedDoc ? (
              <Empty
                description={docError || 'Select a document to view'}
                image={Empty.PRESENTED_IMAGE_SIMPLE}
              />
            ) : (
              <div style={{ maxHeight: '70vh', overflowY: 'auto', paddingRight: 8 }}>
                <Paragraph style={{ color: 'rgba(255,255,255,0.55)', marginBottom: 16 }}>
                  Source: <Text code>{selectedDoc.path}</Text>
                </Paragraph>
                <Divider style={{ borderColor: '#303030', marginTop: 8 }} />
                <div className="docs-markdown">
                  <ReactMarkdown
                    remarkPlugins={[remarkGfm]}
                    components={{
                      a: ({ href, children, ...props }) => {
                        const link = href || '';
                        const isExternal = /^(https?:|mailto:|tel:)/i.test(link);

                        if (!isExternal) {
                          const resolvedDocId = resolveDocIdFromHref(link, docs);
                          if (resolvedDocId) {
                            return (
                              <a
                                href="#"
                                onClick={(e) => {
                                  e.preventDefault();
                                  setSelectedDocId(resolvedDocId);
                                }}
                                {...props}
                              >
                                {children}
                              </a>
                            );
                          }
                        }

                        return (
                          <a href={link} target={isExternal ? '_blank' : undefined} rel={isExternal ? 'noreferrer' : undefined} {...props}>
                            {children}
                          </a>
                        );
                      },
                    }}
                  >
                    {selectedDoc.content}
                  </ReactMarkdown>
                </div>
              </div>
            )}
          </Card>
        </Col>
      </Row>
    </Space>
  );
}
