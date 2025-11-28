import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { apiService } from '../services/api';
import { LoadingSpinner } from '../components/LoadingSpinner';

function ReadArticle() {
  const { id } = useParams();
  const navigate = useNavigate();
  const [article, setArticle] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [showOriginal, setShowOriginal] = useState(false);
  const [deleting, setDeleting] = useState(false);

  const handleDelete = async () => {
    if (!window.confirm('Tem certeza que deseja remover este artigo da lista de leitura?')) {
      return;
    }
    
    setDeleting(true);
    try {
      await apiService.deleteFromReadingList(id);
      navigate('/articles', { state: { deleted: true, deletedId: parseInt(id) } });
    } catch (err) {
      console.error('Erro ao excluir artigo:', err);
      alert('Erro ao excluir artigo. Tente novamente.');
    } finally {
      setDeleting(false);
    }
  };

  useEffect(() => {
    fetchArticle();
  }, [id]);

  const fetchArticle = async () => {
    try {
      setLoading(true);
      const data = await apiService.getFromReadingList(id);
      setArticle(data);
    } catch (err) {
      console.error('Erro ao carregar artigo:', err);
      setError('Artigo não encontrado na lista de leitura');
    } finally {
      setLoading(false);
    }
  };

  const formatDate = (dateString) => {
    if (!dateString) return '-';
    const date = new Date(dateString);
    if (isNaN(date.getTime())) return '-';
    return date.toLocaleDateString('pt-BR', {
      day: '2-digit',
      month: '2-digit',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    });
  };

  const formatNewsletterName = (from) => {
    if (!from) return '-';
    return from.replace(/<[^>]+>/g, '').trim() || from;
  };

  if (loading) {
    return (
      <div className="flex justify-center items-center min-h-64">
        <LoadingSpinner />
      </div>
    );
  }

  if (error || !article) {
    return (
      <div className="container mx-auto px-4 py-8">
        <div className="bg-white rounded-lg shadow p-8 text-center">
          <div className="text-red-500 mb-4">
            <svg className="h-16 w-16 mx-auto" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            </svg>
          </div>
          <h2 className="text-xl font-bold text-gray-800 mb-2">Artigo não encontrado</h2>
          <p className="text-gray-600 mb-6">{error || 'Este artigo não está na sua lista de leitura.'}</p>
          <button
            onClick={() => navigate('/articles')}
            className="px-6 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition-colors"
          >
            Voltar para Artigos
          </button>
        </div>
      </div>
    );
  }

  const hasContent = article.content && article.content.length > 0;

  return (
    <div className="container mx-auto px-4 py-8 max-w-4xl">
      {/* Navegação */}
      <div className="mb-6">
        <button
          onClick={() => navigate('/articles')}
          className="inline-flex items-center text-blue-600 hover:text-blue-800 transition-colors"
        >
          <svg className="h-5 w-5 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M15 19l-7-7 7-7" />
          </svg>
          Voltar para Artigos
        </button>
      </div>

      {/* Card do Artigo */}
      <div className="bg-white rounded-lg shadow-lg overflow-hidden">
        {/* Header com badge de importado */}
        <div className="bg-gradient-to-r from-green-500 to-green-600 px-6 py-4">
          <div className="flex items-center justify-between">
            <span className="inline-flex items-center px-3 py-1 bg-white/20 text-white text-sm font-medium rounded-full">
              <svg className="h-4 w-4 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M5 13l4 4L19 7" />
              </svg>
              Na Lista de Leitura
            </span>
            <span className="text-white/80 text-sm">
              Importado em {formatDate(article.imported_at)}
            </span>
          </div>
        </div>

        {/* Conteúdo */}
        <div className="p-6">
          {/* Título */}
          <h1 className="text-2xl md:text-3xl font-bold text-gray-900 mb-4 leading-tight">
            {article.title || 'Sem título'}
          </h1>

          {/* Metadados */}
          <div className="flex flex-wrap gap-4 mb-6 text-sm text-gray-600">
            <div className="flex items-center">
              <svg className="h-4 w-4 mr-2 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M16 12a4 4 0 10-8 0 4 4 0 008 0zm0 0v1.5a2.5 2.5 0 005 0V12a9 9 0 10-9 9m4.5-1.206a8.959 8.959 0 01-4.5 1.207" />
              </svg>
              <span>{formatNewsletterName(article.newsletter)}</span>
            </div>
            <div className="flex items-center">
              <svg className="h-4 w-4 mr-2 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
              </svg>
              <span>{formatDate(article.email_date)}</span>
            </div>
            <div className="flex items-center">
              <svg className="h-4 w-4 mr-2 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9" />
              </svg>
              <span>{article.domain}</span>
            </div>
            {hasContent && (
              <div className="flex items-center text-green-600">
                <svg className="h-4 w-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                <span>Conteúdo salvo ({Math.round(article.content.length / 1024)}KB)</span>
              </div>
            )}
          </div>

          {/* Descrição */}
          {article.description && (
            <div className="mb-6 p-4 bg-gray-50 rounded-lg border-l-4 border-blue-500">
              <p className="text-gray-700 italic">{article.description}</p>
            </div>
          )}

          {/* Conteúdo do Artigo */}
          {hasContent && (
            <div className="mb-6">
              <div className="flex items-center justify-between mb-4">
                <h2 className="text-lg font-semibold text-gray-800">Conteúdo do Artigo</h2>
                <button
                  onClick={() => setShowOriginal(!showOriginal)}
                  className="text-sm text-blue-600 hover:text-blue-800 transition-colors"
                >
                  {showOriginal ? 'Voltar para leitura' : 'Ver artigo original'}
                </button>
              </div>
              
              {showOriginal ? (
                <div className="border rounded-lg overflow-hidden">
                  <iframe
                    src={article.url}
                    className="w-full h-[600px]"
                    title="Artigo Original"
                    sandbox="allow-same-origin allow-scripts"
                  />
                </div>
              ) : (
                <div 
                  className="prose prose-lg max-w-none bg-white border rounded-lg p-6 overflow-x-auto article-content"
                  dangerouslySetInnerHTML={{ __html: article.content }}
                />
              )}
            </div>
          )}

          {/* Mensagem quando não há conteúdo */}
          {!hasContent && (
            <div className="mb-6 p-6 bg-yellow-50 rounded-lg border border-yellow-200">
              <div className="flex items-start">
                <svg className="h-6 w-6 text-yellow-500 mt-0.5 mr-3 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                </svg>
                <div>
                  <h3 className="text-sm font-medium text-yellow-800">Conteúdo não disponível</h3>
                  <p className="mt-1 text-sm text-yellow-700">
                    Não foi possível extrair o conteúdo deste artigo. Isso pode acontecer com sites que bloqueiam scraping ou requerem autenticação.
                    Clique no botão abaixo para abrir o artigo original.
                  </p>
                </div>
              </div>
            </div>
          )}

          {/* URL */}
          <div className="mb-6 p-4 bg-gray-100 rounded-lg">
            <p className="text-sm text-gray-500 mb-1">Link do artigo:</p>
            <a 
              href={article.url} 
              target="_blank" 
              rel="noopener noreferrer"
              className="text-blue-600 hover:text-blue-800 break-all"
            >
              {article.url}
            </a>
          </div>

          {/* Ações */}
          <div className="flex flex-wrap gap-3 pt-4 border-t">
            <a
              href={article.url}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center px-6 py-3 bg-blue-500 text-white font-medium rounded-lg hover:bg-blue-600 transition-colors"
            >
              <svg className="h-5 w-5 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
              </svg>
              Abrir Artigo Original
            </a>
            <button
              onClick={() => navigate('/articles')}
              className="inline-flex items-center px-6 py-3 bg-gray-200 text-gray-700 font-medium rounded-lg hover:bg-gray-300 transition-colors"
            >
              <svg className="h-5 w-5 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M4 6h16M4 10h16M4 14h16M4 18h16" />
              </svg>
              Ver Mais Artigos
            </button>
            <button
              onClick={handleDelete}
              disabled={deleting}
              className="inline-flex items-center px-6 py-3 bg-red-500 text-white font-medium rounded-lg hover:bg-red-600 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {deleting ? (
                <>
                  <svg className="animate-spin h-5 w-5 mr-2" viewBox="0 0 24 24">
                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                  </svg>
                  Excluindo...
                </>
              ) : (
                <>
                  <svg className="h-5 w-5 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                  </svg>
                  Excluir da Lista
                </>
              )}
            </button>
          </div>
        </div>
      </div>

      {/* Info adicional */}
      <div className="mt-6 p-4 bg-blue-50 rounded-lg border border-blue-200">
        <div className="flex items-start">
          <svg className="h-5 w-5 text-blue-500 mt-0.5 mr-3 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <div className="text-sm text-blue-800">
            <p className="font-medium mb-1">Sobre a Lista de Leitura</p>
            <p>Este artigo foi salvo na sua lista de leitura local (NoSQL). Os dados são armazenados de forma segura no seu servidor e persistem entre sessões.</p>
          </div>
        </div>
      </div>

      {/* Estilos para o conteúdo do artigo */}
      <style>{`
        .article-content img {
          max-width: 100%;
          height: auto;
          border-radius: 0.5rem;
          margin: 1rem 0;
        }
        .article-content pre {
          background: #1f2937;
          color: #e5e7eb;
          padding: 1rem;
          border-radius: 0.5rem;
          overflow-x: auto;
          margin: 1rem 0;
        }
        .article-content code {
          background: #f3f4f6;
          padding: 0.125rem 0.25rem;
          border-radius: 0.25rem;
          font-size: 0.875em;
        }
        .article-content pre code {
          background: transparent;
          padding: 0;
        }
        .article-content h1, .article-content h2, .article-content h3, .article-content h4 {
          margin-top: 1.5rem;
          margin-bottom: 0.75rem;
          font-weight: 600;
          line-height: 1.25;
        }
        .article-content h1 { font-size: 1.875rem; }
        .article-content h2 { font-size: 1.5rem; }
        .article-content h3 { font-size: 1.25rem; }
        .article-content p {
          margin-bottom: 1rem;
          line-height: 1.75;
        }
        .article-content ul, .article-content ol {
          margin-bottom: 1rem;
          padding-left: 1.5rem;
        }
        .article-content li {
          margin-bottom: 0.5rem;
        }
        .article-content blockquote {
          border-left: 4px solid #3b82f6;
          padding-left: 1rem;
          margin: 1rem 0;
          font-style: italic;
          color: #6b7280;
        }
        .article-content a {
          color: #3b82f6;
          text-decoration: underline;
        }
        .article-content a:hover {
          color: #1d4ed8;
        }
      `}</style>
    </div>
  );
}

export default ReadArticle;
