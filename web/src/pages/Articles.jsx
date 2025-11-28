import { useState, useEffect, useMemo } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { apiService } from '../services/api';
import { LoadingSpinner } from '../components/LoadingSpinner';
import { Toast } from '../components/Toast';
import { getAuthToken } from '../utils/storage';

const ITEMS_PER_PAGE_OPTIONS = [10, 25, 50, 100];

function Articles() {
  const navigate = useNavigate();
  const location = useLocation();
  const [links, setLinks] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [toast, setToast] = useState(null);
  const [selectedIds, setSelectedIds] = useState(new Set());
  const [deleting, setDeleting] = useState(false);
  const [importedIds, setImportedIds] = useState(new Set());
  const [importingId, setImportingId] = useState(null);
  
  // Filtros
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedNewsletter, setSelectedNewsletter] = useState('');
  const [dateFilter, setDateFilter] = useState('');
  const [showOnlySaved, setShowOnlySaved] = useState(false);

  // Paginação
  const [currentPage, setCurrentPage] = useState(1);
  const [itemsPerPage, setItemsPerPage] = useState(25);

  // Verificar autenticação
  useEffect(() => {
    const token = getAuthToken();
    if (!token) {
      navigate('/login');
    }
  }, [navigate]);

  useEffect(() => {
    fetchLinks();
    fetchImportedIds();
  }, []);

  // Verificar se voltou da página de leitura após exclusão
  useEffect(() => {
    if (location.state?.deleted && location.state?.deletedId) {
      // Remover o ID da lista de importados
      setImportedIds(prev => {
        const newSet = new Set(prev);
        newSet.delete(location.state.deletedId);
        return newSet;
      });
      setToast({ message: 'Artigo removido da lista de leitura!', type: 'success' });
      // Limpar o estado para não mostrar toast novamente
      window.history.replaceState({}, document.title);
    }
  }, [location.state]);

  // Resetar para página 1 quando filtros mudam
  useEffect(() => {
    setCurrentPage(1);
  }, [searchQuery, selectedNewsletter, dateFilter, itemsPerPage, showOnlySaved]);

  const formatDate = (dateString) => {
    if (!dateString) return '-';
    const date = new Date(dateString);
    if (isNaN(date.getTime())) return '-';
    return date.toLocaleDateString('pt-BR', {
      day: '2-digit',
      month: '2-digit',
      year: 'numeric'
    });
  };

  const formatNewsletterName = (from) => {
    if (!from) return '-';
    return from.replace(/<[^>]+>/g, '').trim() || from;
  };

  const fetchLinks = async () => {
    try {
      setLoading(true);
      const data = await apiService.getAllLinks(1, 50000);
      setLinks(data?.links || []);
      setSelectedIds(new Set());
    } catch (err) {
      setError('Erro ao carregar artigos');
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const fetchImportedIds = async () => {
    try {
      const data = await apiService.getImportedIDs();
      setImportedIds(new Set(data?.imported_ids || []));
    } catch (err) {
      console.error('Erro ao buscar IDs importados:', err);
    }
  };

  // Importar artigo para lista de leitura
  const handleImport = async (link) => {
    setImportingId(link.id);
    try {
      await apiService.importToReadingList({
        id: link.id,
        url: link.url,
        title: link.title,
        description: link.description,
        domain: link.domain,
        newsletter: link.newsletter,
        email_date: link.email_date,
        folder: link.folder,
      });
      
      setImportedIds(prev => new Set([...prev, link.id]));
      setToast({ message: 'Artigo importado para lista de leitura!', type: 'success' });
    } catch (err) {
      console.error('Erro ao importar artigo:', err);
      setToast({ message: 'Erro ao importar artigo', type: 'error' });
    } finally {
      setImportingId(null);
    }
  };

  // Abrir artigo salvo (já importado) - navega para a página de leitura
  const handleRead = (link) => {
    navigate(`/read/${link.id}`);
  };

  // Extrair lista única de newsletters
  const newsletters = useMemo(() => {
    const uniqueNewsletters = new Set();
    links.forEach(link => {
      const name = formatNewsletterName(link.newsletter);
      if (name && name !== '-') {
        uniqueNewsletters.add(name);
      }
    });
    return Array.from(uniqueNewsletters).sort();
  }, [links]);

  // Filtrar links
  const filteredLinks = useMemo(() => {
    return links.filter(link => {
      // Filtro de artigos salvos localmente
      if (showOnlySaved && !importedIds.has(link.id)) {
        return false;
      }

      // Filtro de busca por texto (suporta múltiplas palavras separadas por vírgula ou espaço)
      if (searchQuery) {
        const title = (link.title || '').toLowerCase();
        const url = (link.url || '').toLowerCase();
        const newsletterName = formatNewsletterName(link.newsletter).toLowerCase();
        const searchText = `${title} ${url} ${newsletterName}`;
        
        // Separar palavras por vírgula OU espaço e verificar se TODAS existem no texto
        const searchTerms = searchQuery
          .split(/[,\s]+/)  // Divide por vírgula ou espaços
          .map(term => term.trim().toLowerCase())
          .filter(term => term.length > 0); // Remove termos vazios
        
        // Se não há termos válidos, não filtra
        if (searchTerms.length === 0) {
          return true;
        }
        
        // Todas as palavras devem estar presentes (em qualquer ordem)
        const allTermsMatch = searchTerms.every(term => searchText.includes(term));
        if (!allTermsMatch) {
          return false;
        }
      }

      // Filtro por newsletter
      if (selectedNewsletter) {
        const name = formatNewsletterName(link.newsletter);
        if (name !== selectedNewsletter) {
          return false;
        }
      }

      // Filtro por data
      if (dateFilter) {
        const linkDate = link.email_date || link.created_at;
        if (linkDate) {
          const date = new Date(linkDate).toISOString().split('T')[0];
          if (date !== dateFilter) {
            return false;
          }
        }
      }

      return true;
    });
  }, [links, searchQuery, selectedNewsletter, dateFilter, showOnlySaved, importedIds]);

  // Calcular paginação
  const totalPages = Math.ceil(filteredLinks.length / itemsPerPage);
  const startIndex = (currentPage - 1) * itemsPerPage;
  const endIndex = startIndex + itemsPerPage;
  const paginatedLinks = filteredLinks.slice(startIndex, endIndex);

  // Gerar array de páginas para exibição
  const getPageNumbers = () => {
    const pages = [];
    const maxVisiblePages = 5;
    
    if (totalPages <= maxVisiblePages) {
      for (let i = 1; i <= totalPages; i++) {
        pages.push(i);
      }
    } else {
      pages.push(1);
      
      let startPage = Math.max(2, currentPage - 1);
      let endPage = Math.min(totalPages - 1, currentPage + 1);
      
      if (currentPage <= 3) {
        endPage = 4;
      }
      
      if (currentPage >= totalPages - 2) {
        startPage = totalPages - 3;
      }
      
      if (startPage > 2) {
        pages.push('...');
      }
      
      for (let i = startPage; i <= endPage; i++) {
        pages.push(i);
      }
      
      if (endPage < totalPages - 1) {
        pages.push('...');
      }
      
      pages.push(totalPages);
    }
    
    return pages;
  };

  const handleSelectAll = (e) => {
    if (e.target.checked) {
      const pageIds = new Set(paginatedLinks.map(link => link.id));
      setSelectedIds(pageIds);
    } else {
      setSelectedIds(new Set());
    }
  };

  const handleSelectOne = (id) => {
    const newSelected = new Set(selectedIds);
    if (newSelected.has(id)) {
      newSelected.delete(id);
    } else {
      newSelected.add(id);
    }
    setSelectedIds(newSelected);
  };

  const handleBatchDelete = async () => {
    if (selectedIds.size === 0) return;

    const confirmMessage = `Tem certeza que deseja excluir ${selectedIds.size} artigo(s) selecionado(s)?`;
    if (!window.confirm(confirmMessage)) return;

    setDeleting(true);
    let successCount = 0;
    let errorCount = 0;

    for (const id of selectedIds) {
      try {
        await apiService.deleteLink(id);
        successCount++;
      } catch (err) {
        console.error(`Erro ao excluir artigo ${id}:`, err);
        errorCount++;
      }
    }

    if (successCount > 0) {
      setLinks(links.filter(link => !selectedIds.has(link.id)));
      setSelectedIds(new Set());
      
      if (errorCount > 0) {
        setToast({ 
          message: `${successCount} artigo(s) excluído(s), ${errorCount} erro(s)`, 
          type: 'warning' 
        });
      } else {
        setToast({ 
          message: `${successCount} artigo(s) excluído(s) com sucesso!`, 
          type: 'success' 
        });
      }
      
      fetchLinks();
    } else {
      setToast({ message: 'Erro ao excluir artigos', type: 'error' });
    }

    setDeleting(false);
  };

  const clearFilters = () => {
    setSearchQuery('');
    setSelectedNewsletter('');
    setDateFilter('');
    setShowOnlySaved(false);
    setCurrentPage(1);
  };

  const goToPage = (page) => {
    if (page >= 1 && page <= totalPages) {
      setCurrentPage(page);
      setSelectedIds(new Set());
    }
  };

  if (loading) {
    return (
      <div className="flex justify-center items-center min-h-64">
        <LoadingSpinner />
      </div>
    );
  }

  if (error) {
    return (
      <div className="text-center text-red-500 p-8">
        <p>{error}</p>
        <button 
          onClick={fetchLinks}
          className="mt-4 px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600"
        >
          Tentar novamente
        </button>
      </div>
    );
  }

  const isAllSelected = paginatedLinks.length > 0 && paginatedLinks.every(link => selectedIds.has(link.id));
  const hasSelection = selectedIds.size > 0;
  const hasFilters = searchQuery || selectedNewsletter || dateFilter || showOnlySaved;

  return (
    <div className="container mx-auto px-4 py-8">
      {toast && (
        <Toast 
          message={toast.message} 
          type={toast.type} 
          onClose={() => setToast(null)} 
        />
      )}
      
      {/* Cabeçalho */}
      <div className="flex justify-between items-center mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-800">
            Artigos Extraídos ({filteredLinks.length}{filteredLinks.length !== links.length ? ` de ${links.length}` : ''})
          </h1>
          <p className="text-sm text-gray-500 mt-1">
            {importedIds.size} artigo(s) na lista de leitura
          </p>
        </div>
        
        {hasSelection && (
          <button
            onClick={handleBatchDelete}
            disabled={deleting}
            className="px-4 py-2 bg-red-500 text-white rounded hover:bg-red-600 disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
          >
            {deleting ? (
              <>
                <svg className="animate-spin h-4 w-4" viewBox="0 0 24 24">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                </svg>
                Excluindo...
              </>
            ) : (
              <>
                <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                </svg>
                Excluir Selecionados ({selectedIds.size})
              </>
            )}
          </button>
        )}
      </div>

      {/* Filtros */}
      <div className="bg-white rounded-lg shadow p-4 mb-6">
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Buscar
            </label>
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="claude code (palavras em qualquer ordem)"
              className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Newsletter
            </label>
            <select
              value={selectedNewsletter}
              onChange={(e) => setSelectedNewsletter(e.target.value)}
              className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
            >
              <option value="">Todas as newsletters</option>
              {newsletters.map((newsletter) => (
                <option key={newsletter} value={newsletter}>
                  {newsletter}
                </option>
              ))}
            </select>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Data
            </label>
            <input
              type="date"
              value={dateFilter}
              onChange={(e) => setDateFilter(e.target.value)}
              className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
            />
          </div>

          <div className="flex items-end">
            {hasFilters && (
              <button
                onClick={clearFilters}
                className="w-full px-4 py-2 text-gray-600 bg-gray-100 rounded-md hover:bg-gray-200 transition-colors"
              >
                Limpar Filtros
              </button>
            )}
          </div>
        </div>

        {/* Checkbox para filtrar apenas salvos */}
        <div className="mt-4 pt-4 border-t border-gray-200">
          <label className="inline-flex items-center cursor-pointer">
            <input
              type="checkbox"
              checked={showOnlySaved}
              onChange={(e) => setShowOnlySaved(e.target.checked)}
              className="h-4 w-4 text-green-600 rounded border-gray-300 focus:ring-green-500 cursor-pointer"
            />
            <span className="ml-2 text-sm text-gray-700">
              Mostrar apenas artigos salvos localmente
            </span>
            <span className="ml-2 px-2 py-0.5 bg-green-100 text-green-800 text-xs font-medium rounded-full">
              {importedIds.size}
            </span>
          </label>
        </div>
      </div>

      {filteredLinks.length === 0 ? (
        <div className="text-center text-gray-500 py-12 bg-white rounded-lg shadow">
          {hasFilters ? (
            <>
              <p>Nenhum artigo encontrado com os filtros aplicados.</p>
              <button
                onClick={clearFilters}
                className="mt-4 px-4 py-2 text-blue-600 hover:text-blue-800"
              >
                Limpar filtros
              </button>
            </>
          ) : (
            <>
              <p>Nenhum artigo encontrado.</p>
              <p className="text-sm mt-2">Importe emails para ver os artigos extraídos.</p>
            </>
          )}
        </div>
      ) : (
        <>
          {/* Tabela */}
          <div className="overflow-x-auto bg-white rounded-lg shadow">
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50">
                <tr>
                  <th className="px-4 py-3 text-left">
                    <input
                      type="checkbox"
                      checked={isAllSelected}
                      onChange={handleSelectAll}
                      className="h-4 w-4 text-blue-600 rounded border-gray-300 focus:ring-blue-500 cursor-pointer"
                      title="Selecionar todos desta página"
                    />
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Data
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Newsletter
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Título do Artigo
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                    Ações
                  </th>
                </tr>
              </thead>
              <tbody className="bg-white divide-y divide-gray-200">
                {paginatedLinks.map((link) => {
                  const isImported = importedIds.has(link.id);
                  const isImporting = importingId === link.id;
                  
                  return (
                    <tr 
                      key={link.id} 
                      className={`hover:bg-gray-50 ${selectedIds.has(link.id) ? 'bg-blue-50' : ''} ${isImported ? 'bg-green-50' : ''}`}
                    >
                      <td className="px-4 py-4">
                        <input
                          type="checkbox"
                          checked={selectedIds.has(link.id)}
                          onChange={() => handleSelectOne(link.id)}
                          className="h-4 w-4 text-blue-600 rounded border-gray-300 focus:ring-blue-500 cursor-pointer"
                        />
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                        {formatDate(link.email_date || link.created_at)}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                        {formatNewsletterName(link.newsletter)}
                      </td>
                      <td className="px-6 py-4 text-sm text-gray-900 max-w-md">
                        <span className="line-clamp-2" title={link.title || link.url}>
                          {link.title || 'Sem título'}
                        </span>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm">
                        <div className="flex items-center gap-2">
                          {/* Botão Abrir Link (sempre visível) */}
                          <a 
                            href={link.url} 
                            target="_blank" 
                            rel="noopener noreferrer"
                            className="inline-flex items-center px-3 py-1 bg-blue-500 text-white text-xs font-medium rounded hover:bg-blue-600 transition-colors"
                            title="Abrir artigo original"
                          >
                            <svg className="h-3 w-3 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                            </svg>
                            Abrir
                          </a>
                          
                          {isImported ? (
                            // Botão Ler (artigo já importado) - abre página de leitura
                            <button
                              onClick={() => handleRead(link)}
                              className="inline-flex items-center px-3 py-1 bg-green-500 text-white text-xs font-medium rounded hover:bg-green-600 transition-colors"
                              title="Ver artigo salvo"
                            >
                              <svg className="mr-1 h-3 w-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253" />
                              </svg>
                              Ler
                            </button>
                          ) : (
                            // Botão Importar
                            <button
                              onClick={() => handleImport(link)}
                              disabled={isImporting}
                              className="inline-flex items-center px-3 py-1 bg-purple-500 text-white text-xs font-medium rounded hover:bg-purple-600 transition-colors disabled:opacity-50"
                              title="Salvar na lista de leitura"
                            >
                              {isImporting ? (
                                <>
                                  <svg className="animate-spin mr-1 h-3 w-3" viewBox="0 0 24 24">
                                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
                                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                                  </svg>
                                  Importando...
                                </>
                              ) : (
                                <>
                                  <svg className="mr-1 h-3 w-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 4v16m8-8H4" />
                                  </svg>
                                  Importar
                                </>
                              )}
                            </button>
                          )}
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>

          {/* Paginação */}
          <div className="mt-4 bg-white rounded-lg shadow px-4 py-3">
            <div className="flex flex-col sm:flex-row justify-between items-center gap-4">
              <div className="flex items-center gap-4 text-sm text-gray-600">
                <span>
                  Mostrando {startIndex + 1} - {Math.min(endIndex, filteredLinks.length)} de {filteredLinks.length}
                </span>
                <div className="flex items-center gap-2">
                  <span>Itens por página:</span>
                  <select
                    value={itemsPerPage}
                    onChange={(e) => setItemsPerPage(Number(e.target.value))}
                    className="px-2 py-1 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                  >
                    {ITEMS_PER_PAGE_OPTIONS.map((option) => (
                      <option key={option} value={option}>
                        {option}
                      </option>
                    ))}
                  </select>
                </div>
              </div>

              {totalPages > 1 && (
                <div className="flex items-center gap-1">
                  <button
                    onClick={() => goToPage(currentPage - 1)}
                    disabled={currentPage === 1}
                    className="px-3 py-1 rounded-md border border-gray-300 text-gray-600 hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M15 19l-7-7 7-7" />
                    </svg>
                  </button>

                  {getPageNumbers().map((page, index) => (
                    <button
                      key={index}
                      onClick={() => typeof page === 'number' && goToPage(page)}
                      disabled={page === '...'}
                      className={`px-3 py-1 rounded-md min-w-[36px] ${
                        page === currentPage
                          ? 'bg-blue-500 text-white'
                          : page === '...'
                          ? 'cursor-default text-gray-400'
                          : 'border border-gray-300 text-gray-600 hover:bg-gray-50'
                      }`}
                    >
                      {page}
                    </button>
                  ))}

                  <button
                    onClick={() => goToPage(currentPage + 1)}
                    disabled={currentPage === totalPages}
                    className="px-3 py-1 rounded-md border border-gray-300 text-gray-600 hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M9 5l7 7-7 7" />
                    </svg>
                  </button>
                </div>
              )}
            </div>
          </div>
        </>
      )}
    </div>
  );
}

export default Articles;
