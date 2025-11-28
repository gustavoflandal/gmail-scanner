import { Link } from 'react-router-dom';

export const Header = () => {
  return (
    <header className="bg-primary-600 text-white shadow-lg">
      <div className="max-w-7xl mx-auto px-4 py-4">
        <div className="flex items-center justify-between">
          <Link to="/" className="flex items-center space-x-3">
            <div className="w-8 h-8 bg-white rounded-lg flex items-center justify-center">
              <span className="text-primary-600 font-bold">📧</span>
            </div>
            <h1 className="text-2xl font-bold">Gmail Scanner</h1>
          </Link>

          <nav className="flex items-center space-x-6">
            <Link to="/dashboard" className="hover:bg-primary-700 px-3 py-2 rounded-md transition">
              Dashboard
            </Link>
            <Link to="/articles" className="hover:bg-primary-700 px-3 py-2 rounded-md transition">
              📎 Artigos
            </Link>
            <Link to="/login" className="bg-white text-primary-600 px-4 py-2 rounded-md font-semibold hover:bg-gray-100 transition">
              Login
            </Link>
          </nav>
        </div>
      </div>
    </header>
  );
};
