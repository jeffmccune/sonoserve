import {useEffect} from 'react';
import {useHistory} from '@docusaurus/router';

export default function Home() {
  const history = useHistory();
  
  useEffect(() => {
    history.push('/ui/docs/controller');
  }, [history]);
  
  return null;
}
